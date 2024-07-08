package blueberry

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

type TaskParams map[string]interface{}

func (t TaskParams) GetInt(key string) (int, error) {
	value, exists := t[key]
	if !exists {
		return 0, fmt.Errorf("key %s not found", key)
	}

	intValue, err := convertToInt(value)
	if err != nil {
		return 0, err
	}
	return intValue, nil
}

func (t TaskParams) GetString(key string) (string, error) {
	value, exists := t[key]
	if !exists {
		return "", fmt.Errorf("key %s not found", key)
	}

	strValue, err := convertToString(value)
	if err != nil {
		return "", err
	}
	return strValue, nil
}

func (t TaskParams) GetBool(key string) (bool, error) {
	value, exists := t[key]
	if !exists {
		return false, fmt.Errorf("key %s not found", key)
	}

	boolValue, err := convertToBool(value)
	if err != nil {
		return false, err
	}
	return boolValue, nil
}

func (t TaskParams) GetFloat(key string) (float64, error) {
	value, exists := t[key]
	if !exists {
		return 0.0, fmt.Errorf("key %s not found", key)
	}

	floatValue, err := convertToFloat(value)
	if err != nil {
		return 0.0, err
	}
	return floatValue, nil
}

func convertToInt(value interface{}) (int, error) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int:
		return value.(int), nil
	case reflect.Float64:
		return int(value.(float64)), nil
	case reflect.String:
		intVal, err := strconv.Atoi(value.(string))
		if err != nil {
			return 0, errors.New("value should be convertible to int")
		}
		return intVal, nil
	default:
		return 0, errors.New("value should be of type int")
	}
}

func convertToString(value interface{}) (string, error) {
	if strValue, ok := value.(string); ok {
		return strValue, nil
	}
	return "", errors.New("value should be of type string")
}

func convertToBool(value interface{}) (bool, error) {
	if boolValue, ok := value.(bool); ok {
		return boolValue, nil
	}
	return false, errors.New("value should be of type bool")
}

func convertToFloat(value interface{}) (float64, error) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Float64:
		return value.(float64), nil
	case reflect.Float32:
		return float64(value.(float32)), nil
	case reflect.Int:
		return float64(value.(int)), nil
	case reflect.String:
		floatVal, err := strconv.ParseFloat(value.(string), 64)
		if err != nil {
			return 0.0, errors.New("value should be convertible to float")
		}
		return floatVal, nil
	default:
		return 0.0, errors.New("value should be of type float")
	}
}

// NewTaskParamsFromStruct generates TaskParams from a given struct using tags
func NewTaskParamsFromStruct(s interface{}) (TaskParams, error) {
	params := TaskParams{}
	v := reflect.ValueOf(s)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Tag.Get("task")
		if fieldName == "" {
			fieldName = field.Name
		}
		fieldValue := v.Field(i).Interface()
		fieldType := field.Type.Kind()

		switch fieldType {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			params[fieldName] = fieldValue
		case reflect.Bool:
			params[fieldName] = fieldValue
		case reflect.String:
			params[fieldName] = fieldValue
		case reflect.Float32, reflect.Float64:
			params[fieldName] = fieldValue
		default:
			return nil, fmt.Errorf("unsupported type for field %s: %s", fieldName, fieldType)
		}
	}

	return params, nil
}
