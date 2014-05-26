// Package binding deserializes data from HTTP requests into a struct
// ready for your application to use (without reflection). It also
// facilitates data validation and error handling.
package binding

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Bind takes data out of the request and deserializes into a struct according
// to the Content-Type of the request. If no Content-Type is specified, there
// better be data in the query string, otherwise an error will be produced.
func Bind(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "form-urlencoded") {
		return Form(req, userStruct)
	}

	if strings.Contains(contentType, "multipart/form-data") {
		return MultipartForm(req, userStruct)
	}

	if strings.Contains(contentType, "json") {
		return Json(req, userStruct)
	}

	if contentType == "" {
		if len(req.URL.Query()) > 0 {
			return Form(req, userStruct)
		} else {
			errs.Add([]string{}, ContentTypeError, "Empty Content-Type")
		}
	} else {
		errs.Add([]string{}, ContentTypeError, "Unsupported Content-Type")
	}

	return errs
}

// Form deserializes form data out of the request into a struct you provide.
// This function invokes data validation after deserialization.
func Form(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	parseErr := req.ParseForm()
	if parseErr != nil {
		errs.Add([]string{}, DeserializationError, parseErr.Error())
		return errs
	}

	fm := userStruct.FieldMap()
	for fieldName, fieldPointer := range fm {
		strs := req.Form[fieldName]

		if len(strs) == 0 {
			continue
		}

		fieldSpec, fieldHasSpec := fieldPointer.(Field)
		if fieldHasSpec {
			fieldPointer = fieldSpec.Target
		}

		errorHandler := func(err error) {
			if err != nil {
				errs.Add([]string{fieldName}, TypeError, err.Error())
			}
		}

		if fieldSpec.Binder != nil {
			errs = fieldSpec.Binder(strs, errs)
			continue
		}

		switch t := fieldPointer.(type) {
		case *uint8:
			val, err := strconv.ParseUint(strs[0], 10, 8)
			errorHandler(err)
			*t = uint8(val)
		case *uint16:
			val, err := strconv.ParseUint(strs[0], 10, 16)
			errorHandler(err)
			*t = uint16(val)
		case *uint32:
			val, err := strconv.ParseUint(strs[0], 10, 32)
			errorHandler(err)
			*t = uint32(val)
		case *uint64:
			val, err := strconv.ParseUint(strs[0], 10, 64)
			errorHandler(err)
			*t = val
		case *int8:
			val, err := strconv.ParseInt(strs[0], 10, 8)
			errorHandler(err)
			*t = int8(val)
		case *int16:
			val, err := strconv.ParseInt(strs[0], 10, 16)
			errorHandler(err)
			*t = int16(val)
		case *int32:
			val, err := strconv.ParseInt(strs[0], 10, 32)
			errorHandler(err)
			*t = int32(val)
		case *int64:
			val, err := strconv.ParseInt(strs[0], 10, 64)
			errorHandler(err)
			*t = val
		case *float32:
			val, err := strconv.ParseFloat(strs[0], 32)
			errorHandler(err)
			*t = float32(val)
		case *[]float32:
			for _, str := range strs {
				val, err := strconv.ParseFloat(str, 32)
				errorHandler(err)
				*t = append(*t, float32(val))
			}
		case *float64:
			val, err := strconv.ParseFloat(strs[0], 64)
			errorHandler(err)
			*t = val
		case *[]float64:
			for _, str := range strs {
				val, err := strconv.ParseFloat(str, 64)
				errorHandler(err)
				*t = append(*t, val)
			}
		case *uint:
			val, err := strconv.ParseUint(strs[0], 10, 0)
			errorHandler(err)
			*t = uint(val)
		case *[]uint:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 0)
				errorHandler(err)
				*t = append(*t, uint(val))
			}
		case *int:
			val, err := strconv.ParseInt(strs[0], 10, 0)
			errorHandler(err)
			*t = int(val)
		case *[]int:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 0)
				errorHandler(err)
				*t = append(*t, int(val))
			}
		case *bool:
			val, err := strconv.ParseBool(strs[0])
			errorHandler(err)
			*t = val
		case *[]bool:
			for _, str := range strs {
				val, err := strconv.ParseBool(str)
				errorHandler(err)
				*t = append(*t, val)
			}
		case *string:
			*t = strs[0]
		case *[]string:
			*t = strs
		case *time.Time:
			timeFormat := TimeFormat
			if fieldSpec.TimeFormat != "" {
				timeFormat = fieldSpec.TimeFormat
			}
			val, err := time.Parse(timeFormat, strs[0])
			errorHandler(err)
			*t = val
		case *[]time.Time:
			timeFormat := TimeFormat
			if fieldSpec.TimeFormat != "" {
				timeFormat = fieldSpec.TimeFormat
			}
			for _, str := range strs {
				val, err := time.Parse(timeFormat, str)
				errorHandler(err)
				*t = append(*t, val)
			}
		default:
			errorHandler(errors.New("Field type is unsupported by the application"))
		}
	}

	errs = append(errs, Validate(req, userStruct)...)

	return errs
}

// MultipartForm reads a multipart form request and deserializes its data into
// a struct you provide. It then calls Form to get the rest of the form data
// out of the request.
// TODO: This implementation is not complete yet
func MultipartForm(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	multipartReader, err := req.MultipartReader()
	if err != nil {
		errs.Add([]string{}, DeserializationError, err.Error())
		return errs
	} else {
		form, parseErr := multipartReader.ReadForm(MaxMemory)
		if parseErr != nil {
			errs.Add([]string{}, DeserializationError, parseErr.Error())
			return errs
		}
		req.MultipartForm = form
	}
	return Form(req, userStruct)
}

// Json deserializes a JSON request body into a struct you specify
// using the standard encoding/json package (which uses reflection).
// This function invokes data validation after deserialization.
func Json(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	if req.Body != nil {
		defer req.Body.Close()
		err := json.NewDecoder(req.Body).Decode(userStruct)
		if err != nil && err != io.EOF {
			errs.Add([]string{}, DeserializationError, err.Error())
			return errs
		}
	} else {
		errs.Add([]string{}, DeserializationError, "Empty request body")
		return errs
	}

	errs = append(errs, Validate(req, userStruct)...)

	return errs
}

// Validate ensures that all conditions have been met on every field in the
// populated struct. Validation should occur after the request has been
// deserialized into the struct.
func Validate(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	fm := userStruct.FieldMap()

	for fieldName, fieldSpec := range fm {
		addRequiredError := func() {
			errs.Add([]string{fieldName}, RequiredError, "Required")
		}

		if field, ok := fieldSpec.(Field); ok {
			if field.Required {
				switch t := field.Target.(type) {
				case *uint8:
					if *t == 0 {
						addRequiredError()
					}
				case *uint16:
					if *t == 0 {
						addRequiredError()
					}
				case *uint32:
					if *t == 0 {
						addRequiredError()
					}
				case *uint64:
					if *t == 0 {
						addRequiredError()
					}
				case *int8:
					if *t == 0 {
						addRequiredError()
					}
				case *int16:
					if *t == 0 {
						addRequiredError()
					}
				case *int32:
					if *t == 0 {
						addRequiredError()
					}
				case *int64:
					if *t == 0 {
						addRequiredError()
					}
				case *float32:
					if *t == 0 {
						addRequiredError()
					}
				case *[]float32:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *float64:
					if *t == 0 {
						addRequiredError()
					}
				case *[]float64:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *uint:
					if *t == 0 {
						addRequiredError()
					}
				case *[]uint:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *int:
					if *t == 0 {
						addRequiredError()
					}
				case *[]int:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *bool:
					if !*t == false {
						addRequiredError()
					}
				case *[]bool:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *string:
					if *t == "" {
						addRequiredError()
					}
				case *[]string:
					if len(*t) == 0 {
						addRequiredError()
					}
				case *time.Time:
					if t.IsZero() {
						addRequiredError()
					}
				case *[]time.Time:
					if len(*t) == 0 {
						addRequiredError()
					}
				}
			}
		}
	}

	if validator, ok := userStruct.(Validator); ok {
		errs = validator.Validate(req, errs)
	}

	return errs
}

type (
	// Only types that are FieldMappers can have request data deserialized into them.
	FieldMapper interface {
		// FieldMap returns a map that keys field names from the request
		// to pointers into which the values will be deserialized.
		FieldMap() FieldMap
	}

	// FieldMap is a map of field names from the request to pointers into
	// which the values will be deserialized.
	FieldMap map[string]interface{}

	// Field describes the properties of a struct field.
	Field struct {
		// Target is the struct field to deserialize into.
		Target interface{}

		// Required indicates whether the field is required. A required
		// field that deserializes into the zero value for that type
		// will generate an error.
		Required bool

		// TimeFormat specifies the time format for time.Time fields.
		TimeFormat string

		// Binder is a function that converts the incoming request value(s)
		// to the field type
		Binder func([]string, Errors) Errors
	}

	// Validator can be implemented by your type to handle some
	// rudimentary request validation separately from your
	// application logic.
	Validator interface {
		// Validate validates that the request is OK. It is recommended
		// that validation be limited to checking values for syntax and
		// semantics, enough to know that you can make sense of the request
		// in your application. For example, you might verify that a credit
		// card number matches a valid pattern, but you probably wouldn't
		// perform an actual credit card authorization here.
		Validate(*http.Request, Errors) Errors
	}
)

var (
	// Maximum amount of memory to use when parsing a multipart form.
	// Set this to whatever value you prefer; default is 10 MB.
	MaxMemory = int64(1024 * 1024 * 10)

	// If no TimeFormat is specified for a time.Time field, this
	// format will be used by default when parsing.
	TimeFormat = time.RFC3339
)

const (
	jsonContentType           = "application/json; charset=utf-8"
	StatusUnprocessableEntity = 422
)
