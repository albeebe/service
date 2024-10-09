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

	"github.com/joho/godotenv"
)

const (
	CursorUpAndClear      = "\033[A\033[2K\r" // Moves the cursor up one line, clears that line, and returns the cursor to the start.
	Reset                 = "\033[0m"         // Resets all attributes (color, bold, etc.) to default terminal settings.
	Bold                  = "\033[1m"         // Sets the text to bold.
	BrightBlackBackground = "\033[100m"       // Sets the background color to bright black (dark gray).
	BrightWhite           = "\033[97m"        // Sets the text color to bright white.
	Gray                  = "\033[90m"        // Sets the text color to gray (bright black).
	Green                 = "\033[32m"        // Sets the text color to green.
	Red                   = "\033[31m"        // Sets the text color to red.
	Yellow                = "\033[33m"        // Sets the text color to yellow.
)

// Initialize populates the provided struct with environment variable values.
// The struct must be passed as a pointer and should have `default:""` tags for all fields
// to specify default values. These defaults are only used in local environments,
// where the user is given the choice to either use the default or provide their own value.
//
// Behavior:
//
//	In local environments:
//	- If any required environment variables are missing or unset (excluding string fields, where empty values are permitted),
//	  the user will be prompted to either accept the default value or input their own value.
//	- The user-provided or default values are saved in a ".env" file for future runs.
//	- After prompting, the application will terminate to allow a fresh run with the new environment settings.
//
//	In production environments:
//	- If any required environment variables are missing or unset (excluding string fields, where empty values are allowed),
//	  the function will return an error. Empty values are considered missing for non-string types (e.g., bool, int64, etc.).
//
// Returns:
//
//	An error if:
//	- The passed struct is not a pointer.
//	- Required environment variables are missing in production (excluding empty strings).
//	- Any required `default` tags are missing.
//	- An unexpected error occurs during the process (e.g., issues reflecting the struct or reading from the environment).
func Initialize(spec interface{}, runningInProduction bool) error {

	// Ensure that the passed value is a pointer to a struct
	s, err := reflectStruct(spec)
	if err != nil {
		return fmt.Errorf("failed to reflect struct: %w", err)
	}

	// Load environment variables from the .env file if it exists
	if err := loadDotEnvFile(); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	// Override struct fields with any environment variables that are present
	areAllEnvVarsSet, err := populateFromEnv(s)
	if err != nil {
		return fmt.Errorf("failed to populate from environment variables: %w", err)
	}

	// Ensure all environment variables are set.
	// Empty string values are allowed for string fields only
	if !areAllEnvVarsSet {
		if runningInProduction {
			return errors.New("in production, all required environment variables must be set (empty values are only allowed for string fields)")
		} else {
			// In local environment, prompt the user to manually enter the environment variables
			if err := promptUserForEnvironmentValues(s); err != nil {
				return fmt.Errorf("failed to prompt user for environment values: %w", err)
			}
			os.Exit(1)
		}
	}

	return nil
}

// loadDotEnvFile checks if the .env file exists, and if so, attempts to load the
// environment variables from it.
func loadDotEnvFile() error {

	// Check if the .env file exists
	exists, err := fileExists(".env")
	if err != nil {
		return fmt.Errorf("failed to check if the .env file exists: %w", err)
	}
	if !exists {
		// No error if .env file doesn't exist
		return nil
	}

	// Load the .env file
	return godotenv.Load(".env")
}

// populateDefaults populates the fields of the provided struct with their default
// values. Each field must have a "default" tag specifying the default value. The
// function assigns the default value to the field based on its type.
//
// Supported types include: bool, float64, int64, and string.
// For each type, the function parses the default value (a string from the tag)
// into the appropriate type and assigns it to the field.
//
// An empty string as the default value is only valid for fields of type string.
// For other types (e.g., bool, int, float64), an empty default value will result
// in an error.
//
// If any field is missing a "default" tag or has an invalid default value for
// its type, the function returns an error.
//
// Note: Fields that are anonymous (embedded structs) are ignored.
func populateDefaults(s reflect.Value) error {

	// Loop through the struct fields and set the default
	fields := reflect.VisibleFields(s.Type())
	for _, field := range fields {
		if field.Anonymous {
			continue
		}

		// Make sure the default value is set in the tag
		defaultValue, ok := field.Tag.Lookup("default")
		if !ok {
			return fmt.Errorf("field '%s' is missing the 'default' tag", field.Name)
		}

		// Set the field to the default value based on its type
		switch field.Type.Kind() {
		case reflect.Bool:
			if defaultValue == "" {
				return fmt.Errorf("field '%s' is missing a default bool value", field.Name)
			}
			switch strings.ToLower(defaultValue) {
			case "true":
				if err := setBool(s, field.Name, true); err != nil {
					return err
				}
			case "false":
				if err := setBool(s, field.Name, false); err != nil {
					return err
				}
			default:
				return fmt.Errorf("field '%s' default value must be 'true' or 'false'", field.Name)
			}
		case reflect.Float64:
			if defaultValue == "" {
				return fmt.Errorf("field '%s' is missing a default float64 value", field.Name)
			}
			floatVal, err := strconv.ParseFloat(defaultValue, 64)
			if err != nil {
				return fmt.Errorf("field '%s' default value must be a float64", field.Name)
			}
			if err := setFloat64(s, field.Name, floatVal); err != nil {
				return err
			}
		case reflect.Int64:
			if defaultValue == "" {
				return fmt.Errorf("field '%s' is missing a default int64 value", field.Name)
			}
			intVal, err := strconv.Atoi(defaultValue)
			if err != nil {
				return fmt.Errorf("field '%s' default value must be an int64", field.Name)
			}
			if err := setInt64(s, field.Name, int64(intVal)); err != nil {
				return err
			}
		case reflect.String:
			if err := setString(s, field.Name, defaultValue); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field '%s' has an unsupported type '%s'", field.Name, field.Type.String())
		}
	}

	return nil
}

// populateFromEnv updates the passed struct with values from environment variables.
// It checks for environment variables whose names match the struct field names
// and assigns them to the corresponding fields in the struct.
// Returns true if every field in the struct was successfully set via an environment variable,
// and false if one or more fields were not set.
//
// Supported types include: bool, float64, int64, int, and string.
// For string fields, an empty string ("") is a valid value and will be assigned if present
// in the environment variables. However, if the environment variable is missing entirely,
// the field will not be set, and the function will return false.
//
// If an unsupported field type is encountered or an invalid value is found for
// any field (e.g., a non-boolean value for a bool field), the function returns an error.
//
// Note: Fields that are anonymous (embedded structs) or unexported are ignored.
func populateFromEnv(s reflect.Value) (bool, error) {

	// Track if all fields were successfully set via environment variables
	allSet := true
	fields := reflect.VisibleFields(s.Type())

	for _, field := range fields {
		if field.Anonymous {
			// Skipping embedded (anonymous) fields, as they are not handled
			continue
		}

		// Ensure the field is settable (in case it is unexported)
		fieldVal := s.FieldByName(field.Name)
		if !fieldVal.CanSet() {
			// Unsettable fields are ignored in this implementation
			allSet = false
			continue
		}

		// Check if an environment variable exists for the field
		value, exists := os.LookupEnv(field.Name)
		if !exists {
			allSet = false
			continue
		}

		// Set the field to the value from the environment variable
		switch field.Type.Kind() {
		case reflect.Bool:
			value = strings.ToLower(value)
			switch value {
			case "true":
				setBool(s, field.Name, true)
			case "false":
				setBool(s, field.Name, false)
			default:
				return false, fmt.Errorf("value '%s' for the environment variable '%s' is not a valid bool", value, field.Name)
			}
		case reflect.Float64:
			floatVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return false, fmt.Errorf("value '%s' for the environment variable '%s' is not a valid float64", value, field.Name)
			}
			setFloat64(s, field.Name, floatVal)
		case reflect.Int64, reflect.Int:
			intVal, err := strconv.Atoi(value)
			if err != nil {
				return false, fmt.Errorf("value '%s' for the environment variable '%s' is not a valid int", value, field.Name)
			}
			setInt64(s, field.Name, int64(intVal))
		case reflect.String:
			setString(s, field.Name, value)
		default:
			return false, fmt.Errorf("field '%s' has an unsupported type '%s'", field.Name, field.Type.String())
		}
	}

	return allSet, nil
}

// promptUserForEnvironmentValues prompts the user to input required environment variable values.
// After collecting the user input, the function saves the environment variables
// to a .env file, ensuring that they can be automatically loaded the next time
// the service is run, facilitating a smoother local development experience.
func promptUserForEnvironmentValues(s reflect.Value) error {

	// Notify the user about missing environment variables
	fmt.Println()
	fmt.Printf("%sMissing Environment Variables%s\n\n", Red, Reset)
	fmt.Printf("%sYou are seeing this message because the service is running locally. In production, an error would have been returned.%s\n\n", Yellow, Reset)
	fmt.Printf("%sTo run this service locally, please provide a value for each environment variable, or press [Enter] to use the default.%s\n\n", BrightWhite, Reset)

	// Prompt user to enter values for each environment variable
	fields := reflect.VisibleFields(s.Type())
	variables := map[string]string{}
	for _, field := range fields {
		if field.Anonymous {
			continue
		}

		// Print out the prompt for each variable
		defaultValue, exists := os.LookupEnv(field.Name)
		if !exists {
			defaultValue = field.Tag.Get("default")
		}
		fmt.Printf("\n%s%s%s%s\n", Reset, Bold, Gray, field.Name)
		fmt.Printf("%s: ", formatInputLine(defaultValue, ""))

		// Handle input based on the field type
		var val string
		switch field.Type.Kind() {
		case reflect.Bool:
			val = getBoolInput(defaultValue)
		case reflect.Float64:
			val = getFloat64Input(defaultValue)
		case reflect.Int64:
			val = getInt64Input(defaultValue)
		case reflect.String:
			val = getStringInput(defaultValue)
		default:
			return fmt.Errorf("field '%s' has an unsupported type '%s'", field.Name, field.Type.String())
		}
		variables[field.Name] = val
		fmt.Printf("%s%s\n", CursorUpAndClear, formatInputLine(variables[field.Name], ""))
	}

	// Generate the .env file
	envFile, err := os.Create(".env")
	if err != nil {
		return err
	}
	defer envFile.Close()
	for variable, value := range variables {
		if _, err := envFile.WriteString(fmt.Sprintf("%s=%s\n", variable, strconv.Quote(value))); err != nil {
			return err
		}
	}

	// Notify the user of successful setup
	fmt.Printf("\n\n%sYour environment has been successfully set up!%s\n\n", Green, Reset)
	fmt.Printf("%sThe environment variables have been saved to the %s.env%s%s file and will be automatically loaded the next time you run the application.%s\n\n", BrightWhite, BrightBlackBackground, Reset, BrightWhite, Reset)

	return nil
}

// formatInputLine formats and returns a string representing a user input line for the console.
// It displays the input value along with an optional error message in a visually structured format.
//
// If an error message is provided, the output will include the error highlighted in red.
// If no error message is given, it simply displays the input value.
func formatInputLine(inputValue string, errorMessage string) string {

	arrow := "└──"
	if len(errorMessage) > 0 {
		// Return formatted string with error message
		return fmt.Sprintf("%s%s %s %s%s%s%s%s %s%s%s%s%s",
			Reset, Gray, arrow, BrightBlackBackground, BrightWhite, inputValue,
			Gray, Reset, Gray, Reset, Red, errorMessage, Reset)
	}

	// Return formatted string without error message
	return fmt.Sprintf("%s%s %s %s%s%s%s",
		Reset, Gray, arrow, BrightBlackBackground, BrightWhite, inputValue, Reset)
}

// readLine reads a single line of text from standard input and returns it.
// If an error occurs during scanning, it returns the error.
func readLine() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", scanner.Err() // Return error if there is a failure
}

// getBoolInput prompts the user to enter 'true' or 'false', or returns the default value if no input is given.
// The function validates that the input is a valid bool and handles incorrect input by prompting the user again.
func getBoolInput(defaultValue string) string {
	for {
		// Get user input
		input, err := readLine()
		if err != nil {
			// If there was an error reading input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter 'true' or 'false'"))
			continue
		}

		// Return default value if the user presses [Enter] without typing anything
		if input == "" {
			return defaultValue
		}

		// Validate user input for 'true' or 'false'
		switch strings.ToLower(input) {
		case "true":
			return "true"
		case "false":
			return "false"
		default:
			// If invalid input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter 'true' or 'false'"))
		}
	}
}

// getFloat64Input prompts the user to input a float64 value, or returns the default value if no input is provided.
// The function validates that the input is a valid float64 and handles incorrect input by prompting the user again.
func getFloat64Input(defaultValue string) string {
	for {
		// Get user input
		input, err := readLine()
		if err != nil {
			// If there was an error reading input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter a valid float64"))
			continue
		}

		// Return default value if the user presses [Enter] without typing anything
		if input == "" {
			return defaultValue
		}

		// Try to parse the input as float64
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			// If invalid input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter a valid float64"))
			continue
		}

		// Return the successfully parsed float64 as a string
		return fmt.Sprintf("%g", val)
	}
}

// getInt64Input prompts the user to input an int64 value, or returns the default value if no input is provided.
// The function validates that the input is a valid int64 and handles incorrect input by prompting the user again.
func getInt64Input(defaultValue string) string {
	for {
		// Get user input
		input, err := readLine()
		if err != nil {
			// If there was an error reading input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter a valid int64"))
			continue
		}

		// Return default value if the user presses [Enter] without typing anything
		if input == "" {
			return defaultValue
		}

		// Try to parse the input as an int64
		val, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			// If invalid input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter a valid int64"))
			continue
		}

		// Return the successfully parsed int64 as a string
		return fmt.Sprintf("%d", val)
	}
}

// getStringInput prompts the user to input a string value, or returns the default value if no input is provided.
// The function validates that the input is a valid string and handles incorrect input by prompting the user again.
func getStringInput(defaultValue string) string {
	for {
		// Read user input
		input, err := readLine()
		if err != nil {
			// If there was an error reading input, prompt again
			fmt.Printf("%s%s: ", CursorUpAndClear, formatInputLine(defaultValue, "Enter a valid string"))
			continue
		}

		// Return default value if the user presses [Enter] without typing anything
		if input == "" {
			return defaultValue
		}

		// Return the valid input string
		return input
	}
}

// fileExists checks if the file exists and returns true if it does.
// Returns false if the file does not exist, and an error for other issues.
func fileExists(filePath string) (exists bool, err error) {
	if _, err = os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// File does not exist, no error
			return false, nil
		}
		// Other error occurred (e.g., permission issues)
		return false, err
	}
	// File exists
	return true, nil
}

// reflectStruct checks if the provided interface is a pointer to a struct.
// If it is, the function returns the reflected Value of the struct.
// Otherwise, it returns an error if the input is not a pointer or if the pointer
// does not point to a struct.
func reflectStruct(spec interface{}) (reflect.Value, error) {
	s := reflect.ValueOf(spec)

	// Check if the input is a pointer
	if s.Kind() != reflect.Ptr {
		return reflect.Value{}, fmt.Errorf("expected a pointer to a struct, but got a non-pointer of kind %s", s.Kind())
	}

	// Dereference the pointer
	s = s.Elem()

	// Check if the dereferenced value is a struct
	if s.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("expected a pointer to a struct, but got a pointer to a non-struct of kind %s", s.Kind())
	}

	// Return the reflected struct value
	return s, nil
}

// setBool sets the value of a struct's boolean field by its name.
// Returns an error if the field does not exist in the struct.
func setBool(spec reflect.Value, fieldName string, value bool) error {
	curField := spec.FieldByName(fieldName)
	if !curField.IsValid() {
		return fmt.Errorf("struct is missing field: %s", fieldName)
	}
	curField.SetBool(value)
	return nil
}

// setFloat64 sets the value of a struct's float64 field by its name.
// Returns an error if the field does not exist in the struct.
func setFloat64(spec reflect.Value, fieldName string, value float64) error {
	curField := spec.FieldByName(fieldName)
	if !curField.IsValid() {
		return fmt.Errorf("struct is missing field: %s", fieldName)
	}
	curField.SetFloat(value)
	return nil
}

// setInt64 sets the value of a struct's int64 field by its name.
// Returns an error if the field does not exist in the struct.
func setInt64(spec reflect.Value, fieldName string, value int64) error {
	curField := spec.FieldByName(fieldName)
	if !curField.IsValid() {
		return fmt.Errorf("struct is missing field: %s", fieldName)
	}
	curField.SetInt(value)
	return nil
}

// setString sets the value of a struct's string field by its name.
// Returns an error if the field does not exist in the struct.
func setString(spec reflect.Value, fieldName, value string) error {
	curField := spec.FieldByName(fieldName)
	if !curField.IsValid() {
		return fmt.Errorf("struct is missing field: %s", fieldName)
	}
	curField.SetString(value)
	return nil
}
