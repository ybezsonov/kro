package main

import "reflect"

type A struct {
	Name string
	Kind string
	Test struct {
		Age   int
		Items []struct {
			Name string
		}
	}
}

func main() {
	a := A{
		Name: "test",
		Kind: "test",
		Test: struct {
			Age   int
			Items []struct {
				Name string
			}
		}{
			Age: 10,
			Items: []struct {
				Name string
			}{
				{
					Name: "test",
				},
			},
		},
	}

	var ia interface{} = a

	v := reflect.ValueOf(ia)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	for _, field := range v.Type().Field() {

	}
}
