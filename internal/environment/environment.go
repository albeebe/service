// Copyright (c) 2024 Alan Beebe [www.alanbeebe.com]
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// Created: September 30, 2024

package environment

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
)

const (
	Reset  = "\033[0m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
)

// Pass in a pointer to a struct, with optional `default:""` tags
// and the struct will automatically be populated with default
// values and/or any environment variables that are present.
//
// Initializing while running locally (vs production) and failing
// to set all the environment variables will result in the user
// being prompted to setup their environment followed immediately
// by the application terminating
//
// Initializing while running in production will return an error if
// every value is not set via an environment variable. Empty values
// are NOT treated as missing
func Initialize(spec interface{}) error {

	// Make sure a pointer to a struct was passed
	s, err := reflectStruct(spec)
	if err != nil {
		return err
	}

	// Load the structs default values
	if err := loadDefaults(s); err != nil {
		return err
	}

	// Override the struct with any environment variables that are present
	allEnvVarsSet, err := loadEnvVars(s)
	if err != nil {
		return err
	}

	// Make sure all the environment variables were set
	if !allEnvVarsSet {
		if runningInProduction() {
			return errors.New("to run in production, every environment variable is required to be set")
		} else {
			if err := promptCreateEnvironment(s); err != nil {
				return err
			}
			os.Exit(0)
		}
	}

	return nil
}

// Returns true if we're currently running inside GCP
func runningInProduction() bool {
	return metadata.OnGCE()
}

// Update the passed struct to use the default values
// Default values are taken from the struct fields tag (Ex: `default:"foobar"`)
func loadDefaults(s reflect.Value) error {

	// Loop through the struct fields and set the default
	fields := reflect.VisibleFields(s.Type())
	for _, field := range fields {
		if field.Anonymous {
			continue
		}

		// Make sure the default value was set
		defaultValue, ok := field.Tag.Lookup("default")
		if !ok {
			return errors.New(fmt.Sprintf("Field '%s' is missing a default value", field.Name))
		}

		// Set the field to the default value
		switch field.Type.Kind() {
		case reflect.Bool:
			if defaultValue == "" {
				return errors.New(fmt.Sprintf("Field '%s' is missing a default value", field.Name))
			}
			switch strings.ToLower(defaultValue) {
			case "true":
				if err := setBool(s, field.Name, true); err != nil {
					return err
				}
			case "false":
				if err := setBool(s, field.Name, true); err != nil {
					return err
				}
			default:
				return errors.New(fmt.Sprintf("The default value '%s' is not a valid bool for field '%s'", defaultValue, field.Name))
			}
		case reflect.Float64:
			if defaultValue == "" {
				return errors.New(fmt.Sprintf("Field '%s' is missing a default value", field.Name))
			}
			floatVal, err := strconv.ParseFloat(defaultValue, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("The default value '%s' is not a valid float64 for field '%s'", defaultValue, field.Name))
			}
			if err := setFloat64(s, field.Name, floatVal); err != nil {
				return err
			}
		case reflect.Int64, reflect.Int:
			if defaultValue == "" {
				return errors.New(fmt.Sprintf("Field '%s' is missing a default value", field.Name))
			}
			intVal, err := strconv.Atoi(defaultValue)
			if err != nil {
				return errors.New(fmt.Sprintf("The default value '%s' is not a valid int64 for field '%s'", defaultValue, field.Name))
			}
			if err := setInt64(s, field.Name, int64(intVal)); err != nil {
				return err
			}
		case reflect.String:
			if err := setString(s, field.Name, defaultValue); err != nil {
				return err
			}
		default:
			return errors.New("unable to set the default value because the field type isn't supported")
		}
	}

	return nil
}

// Update the passed struct to use any environment variables that were set
// Returns TRUE if every field in the struct was set via an environment variable
func loadEnvVars(s reflect.Value) (bool, error) {

	// Loop through the struct fields and use any environment variables that were set
	allSet := true
	fields := reflect.VisibleFields(s.Type())
	for _, field := range fields {
		if field.Anonymous {
			continue
		}

		// Get the fields environment variable key
		envVarKey := field.Tag.Get("envvar")
		if len(envVarKey) == 0 {
			return false, errors.New(fmt.Sprintf("Field '%s' is missing the 'envvar' tag", field.Name))
		}

		// Check if an environment variable exists for the field
		value, exists := os.LookupEnv(envVarKey)
		if !exists {
			allSet = false
			continue
		}

		// Set the field to the value of the environment variable
		switch field.Type.Kind() {
		case reflect.Bool:
			value = strings.ToLower(value)
			switch value {
			case "true":
				setBool(s, field.Name, true)
			case "false":
				setBool(s, field.Name, false)
			default:
				return false, errors.New(fmt.Sprintf("The value '%s' for the environment variable '%s' is not a valid bool", value, envVarKey))
			}
		case reflect.Float64:
			floatVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false, errors.New(fmt.Sprintf("The value '%s' for the environment variable '%s' is not a valid float64", value, envVarKey))
			}
			setFloat64(s, field.Name, floatVal)
		case reflect.Int64, reflect.Int:
			intVal, err := strconv.Atoi(value)
			if err != nil {
				return false, errors.New(fmt.Sprintf("The value '%s' for the environment variable '%s' is not a valid int64", value, envVarKey))
			}
			setInt64(s, field.Name, int64(intVal))
		case reflect.String:
			setString(s, field.Name, value)
		default:
			return false, errors.New("Cannot set fields value, because the field type isn't supported")
		}
	}

	return allSet, nil
}

// Prompt the user to create their environment
func promptCreateEnvironment(s reflect.Value) error {

	// Find the longest field name and default value which we'll use for formatting the display
	longestName := 0
	longestValue := 0
	fields := reflect.VisibleFields(s.Type())
	for _, field := range fields {
		if field.Anonymous {
			continue
		}
		if len(field.Name) > longestName {
			longestName = len(field.Name)
		}
		if len(field.Tag.Get("default")) > longestValue {
			longestValue = len(field.Tag.Get("default"))
		}
	}

	// Notify user they need to set up their environment
	fmt.Println(Red)
	fmt.Println("In order to run this service locally, you must set ALL the environment variables.")
	fmt.Println("Type the value you would like to use, or press enter to use the default.")
	fmt.Println(Reset)

	// Prompt user to enter values for each env var
	variables := map[string]string{}
	for _, field := range fields {
		if field.Anonymous {
			continue
		}

		// Get the fields environment variable key
		envVarKey := field.Tag.Get("envvar")
		if len(envVarKey) == 0 {
			return errors.New(fmt.Sprintf("Field '%s' is missing the 'envvar' tag", field.Name))
		}

		// Print out the prompt
		defaultValue := field.Tag.Get("default")
		totalDots := (longestName + longestValue) - (len(envVarKey) + len(defaultValue)) + 3
		dots := strings.Repeat(".", totalDots)
		fmt.Printf(Reset+"%s"+Gray+" %s "+Reset+"["+Cyan+"%s"+Reset+"]: ", envVarKey, dots, defaultValue)

		// Get the input
		switch field.Type.Kind() {
		case reflect.Bool:
			val, err := inputBool(defaultValue == "true")
			if err != nil {
				return err
			}
			if val {
				variables[envVarKey] = "true"
			} else {
				variables[envVarKey] = "false"
			}
		case reflect.Float64:
			if defaultValue == "" {
				defaultValue = "0"
			}
			floatVal, err := strconv.ParseFloat(defaultValue, 64)
			if err != nil {
				return err
			}
			val, err := inputFloat64(floatVal)
			if err != nil {
				return err
			}
			variables[envVarKey] = fmt.Sprintf("%f", val)
		case reflect.Int64:
			if defaultValue == "" {
				defaultValue = "0"
			}
			intVal, err := strconv.Atoi(defaultValue)
			if err != nil {
				return err
			}
			val, err := inputInt64(int64(intVal))
			if err != nil {
				return err
			}
			variables[envVarKey] = strconv.Itoa(int(val))
		case reflect.String:
			val, err := inputString(defaultValue)
			if err != nil {
				return err
			}
			variables[envVarKey] = val
		default:
			return errors.New(fmt.Sprintf("Field '%s' has an unsupported type '%s'", field.Name, field.Type.String()))
		}
	}

	// Generate the local.env file
	localENV, err := os.Create("local.env")
	if err != nil {
		return err
	}
	defer localENV.Close()
	for variable, value := range variables {
		if _, err := localENV.WriteString(variable + "=" + strconv.Quote(value) + "\n"); err != nil {
			return err
		}
	}

	// Ask the user if they want to replace their existing run.sh file if it exists
	var createBashScript bool
	if _, err := os.Stat("run.sh"); err == nil {
		fmt.Println()
		for {
			fmt.Printf("You already have a " + Yellow + "run.sh" + Reset + " file. Do you want to overwrite it [Yn]: ")
			val, err := inputString("y")
			if err != nil {
				return err
			}
			if strings.ToLower(val) == "y" {
				createBashScript = true
				break
			}
			if strings.ToLower(val) == "n" {
				createBashScript = false
				break
			}
		}
	} else {
		// Script doesn't exist so we want to create it
		createBashScript = true
	}

	// Create (or replace) the run.sh file
	if createBashScript {
		runSH, err := os.Create("run.sh")
		if err != nil {
			return err
		}
		defer runSH.Close()
		if _, err := runSH.WriteString(`#!/bin/bash
set -a
source ./local.env
set +a
go run .`); err != nil {
			return err
		}
	}

	fmt.Println("\n\nYour environment is set up. Run this service via " + Yellow + "sh run.sh\n\n" + Reset)

	return nil
}

// Read a string of text from standard input
func getStandardInput() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", scanner.Err()
}

// Get a bool value from the user, or return the default if nothing was entered
func inputBool(def bool) (bool, error) {
	for {
		input, err := getStandardInput()
		if err != nil {
			return def, err
		}
		if input == "" {
			return def, nil
		}
		switch strings.ToLower(input) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			fmt.Printf("   Type " + Cyan + "true" + Reset + " or " + Cyan + "false" + Reset + ": ")
		}
	}
}

// Get a float64 value from the user, or return the default if nothing was entered
func inputFloat64(def float64) (float64, error) {
	for {
		input, err := getStandardInput()
		if err != nil {
			return def, err
		}
		if input == "" {
			return def, nil
		}
		floatVal, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Printf("   Enter a valid float64: ")
		} else {
			return floatVal, nil
		}
	}
}

// Get an int64 value from the user, or return the default if nothing was entered
func inputInt64(def int64) (int64, error) {
	for {
		input, err := getStandardInput()
		if err != nil {
			return def, err
		}
		if input == "" {
			return def, nil
		}
		intVal, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("   Enter a valid int64: ")
		} else {
			return int64(intVal), nil
		}
	}
}

// Get a bool value from the user, or return the default if nothing was entered
func inputString(def string) (string, error) {
	input, err := getStandardInput()
	if err != nil {
		return def, err
	}
	if input == "" {
		return def, nil
	}
	return input, nil
}

// Reflect the passed interface
// Returns an error if the interface is not a pointer to a struct
func reflectStruct(spec interface{}) (reflect.Value, error) {
	s := reflect.ValueOf(spec)
	if s.Kind() != reflect.Ptr {
		return s, errors.New("only pointers can be passed")
	}
	s = s.Elem()
	if s.Kind() != reflect.Struct {
		return s, errors.New("only structs can be passed")
	}
	return s, nil
}

// Set a struct fields bool value by using the fields name
func setBool(spec reflect.Value, field string, value bool) error {
	curField := spec.FieldByName(field)
	if !curField.IsValid() {
		return errors.New("struct is missing field:" + field)
	}
	curField.SetBool(value)
	return nil
}

// Set a struct fields float64 value by using the fields name
func setFloat64(spec reflect.Value, field string, value float64) error {
	curField := spec.FieldByName(field)
	if !curField.IsValid() {
		return errors.New("struct is missing field:" + field)
	}
	curField.SetFloat(value)
	return nil
}

// Set a struct fields int64 value by using the fields name
func setInt64(spec reflect.Value, field string, value int64) error {
	curField := spec.FieldByName(field)
	if !curField.IsValid() {
		return errors.New("struct is missing field:" + field)
	}
	curField.SetInt(value)
	return nil
}

// Set a struct fields string value by using the fields name
func setString(spec reflect.Value, field, value string) error {
	curField := spec.FieldByName(field)
	if !curField.IsValid() {
		return errors.New("struct is missing field:" + field)
	}
	curField.SetString(value)
	return nil
}
