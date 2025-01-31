package apps

import (
	"fmt"
	"reflect"

	models "github.com/lnbits/lnbits/models"
)

type Settings struct {
	Models   []Model                `json:"models"`
	Triggers map[string]interface{} `json:"triggers"` // functions
	Actions  map[string]interface{} `json:"actions"`  // functions
}

func (s Settings) getModel(modelName string) Model {
	for _, m := range s.Models {
		if m.Name == modelName {
			return m
		}
	}
	return Model{}
}

func (s Settings) validate() error {
	validTypes := []string{
		"string",
		"number",
		"boolean",
		"ref",
		"msatoshi",
		"url",
	}

	for m, model := range s.Models {
		if model.Name == "" {
			return fmt.Errorf("models[%d].name is not provided", m)
		}

		if len(model.Fields) == 0 {
			return fmt.Errorf("model %s has no fields", model.Name)
		}

		for f, field := range model.Fields {
			if field.Name == "" && !(field.Computed != nil && field.Display != "") {
				return fmt.Errorf("models[%d].fields[%d].name is not provided", m, f)
			}

			typeIsValid := false
			for _, validType := range validTypes {
				if validType == field.Type {
					typeIsValid = true
					break
				}
			}
			if typeIsValid == false {
				return fmt.Errorf("%s.%s's type cannot be '%s', must be one of %v",
					model.Name, field.Name, field.Type, validTypes)
			}

			if field.Type == "ref" {
				if field.Ref == "" {
					return fmt.Errorf("%s.%s's ref not provided", model.Name, field.Name)
				}

				refExists := false
				for _, refModel := range s.Models {
					if field.Ref == refModel.Name {
						refExists = true
						break
					}
				}
				if refExists == false {
					return fmt.Errorf("%s.%s's ref '%s' doesn't exist",
						model.Name, field.Name, field.Ref)
				}
			}
		}
	}

	return nil
}

type Model struct {
	Name    string      `json:"name"`
	Display string      `json:"display,omitempty"`
	Fields  []Field     `json:"fields"`
	Filter  interface{} `json:"filter"` // in lua this is a function, just check for presence
}

func (m Model) validateItem(item models.AppDataItem) error {
	if len(m.Fields) == 0 {
		return fmt.Errorf("unknown model")
	}

	for fieldName, fieldValue := range item.Value {
		fieldExpected := false
		for _, field := range m.Fields {
			if field.Name == fieldName {
				fieldExpected = true

				switch field.Type {
				case "string", "url":
					if reflect.TypeOf(fieldValue).Name() != "string" {
						return fmt.Errorf("'%v' is not a string", fieldValue)
					}
				case "number":
					if reflect.TypeOf(fieldValue).Name() != "float64" {
						return fmt.Errorf("'%v' is not a number", fieldValue)
					}
				case "msatoshi":
					if reflect.TypeOf(fieldValue).Name() != "float64" {
						return fmt.Errorf("'%v' is not a number", fieldValue)
					}
					msat := int64(fieldValue.(float64))
					if float64(msat) != fieldValue.(float64) {
						return fmt.Errorf(
							"'%v' is not an integer, msatoshi must be integer", fieldValue)
					}
					if msat > 100000000000 {
						return fmt.Errorf("'%v' is way too many satoshis", fieldValue)
					}
				case "boolean":
					if reflect.TypeOf(fieldValue).ConvertibleTo(booltype) {
						return fmt.Errorf("'%v' is not a boolean", fieldValue)
					}
				case "ref":
					if reflect.TypeOf(fieldValue).Name() != "string" {
						return fmt.Errorf("'%v' is not a ref string", fieldValue)
					}
					ref, err := DBGet(
						item.WalletID, item.App, field.Ref, fieldValue.(string))
					if err != nil || ref == nil {
						return fmt.Errorf("'%v' is not a valid ref", fieldValue)
					}
				}

				break
			}
		}
		if fieldExpected == false {
			return fmt.Errorf("unexpected field %s", fieldName)
		}
	}

	return nil
}

type Field struct {
	Name     string      `json:"name"`
	Display  string      `json:"display,omitempty"`
	Type     string      `json:"type"`
	Required bool        `json:"required,omitempty"`
	Default  interface{} `json:"default,omitempty"`
	Ref      string      `json:"ref,omitempty"`
	Hidden   bool        `json:"hidden,omitempty"`
	Computed interface{} `json:"computed,omitempty"` // lua function, like above
}
