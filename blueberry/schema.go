package blueberry

import (
	"fmt"
	"reflect"
)

type TaskParamType string

const (
	TypeInt    TaskParamType = "int"
	TypeBool   TaskParamType = "bool"
	TypeString TaskParamType = "string"
	TypeFloat  TaskParamType = "float"
)

type TaskParamDefinition map[string]TaskParamType

// TaskSchema is used to define the schema for the task
type TaskSchema struct {
	Fields TaskParamDefinition // map[fieldName]fieldType
}

// NewTaskSchema is a helper function to create a new TaskSchema
func NewTaskSchema(fields TaskParamDefinition) TaskSchema {
	return TaskSchema{
		Fields: fields,
	}
}

// NewSchemaFromStruct generates a TaskSchema from a given struct using tags
func NewSchemaFromStruct(s interface{}) (TaskSchema, error) {
	fields := TaskParamDefinition{}
	v := reflect.ValueOf(s)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Tag.Get("task")
		if fieldName == "" {
			fieldName = field.Name
		}
		fieldType := field.Type.Kind()

		switch fieldType {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fields[fieldName] = TypeInt
		case reflect.Bool:
			fields[fieldName] = TypeBool
		case reflect.String:
			fields[fieldName] = TypeString
		case reflect.Float32, reflect.Float64:
			fields[fieldName] = TypeFloat
		default:
			return TaskSchema{}, fmt.Errorf("unsupported type for field %s: %s", fieldName, fieldType)
		}
	}

	return NewTaskSchema(fields), nil
}
