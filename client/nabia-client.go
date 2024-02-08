package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
)

func badSubcommand(message string) string {
	var error string

	error = fmt.Sprintln("Wrong subcommand:", message)
	error += fmt.Sprintln("\nExamples of usage:")
	error += fmt.Sprintf("%s %s %s\n", os.Args[0], "GET", "/path-to-key/xyz")
	error += fmt.Sprintf("%s %s %s %s\n", os.Args[0], "POST", "/path-to-key/abc", "filename.txt")
	error += fmt.Sprintf("\nPlease read the project's documentation at %s\n", "https://git.kydara.com/L1Cafe/nabia-client")

	return error
}

func subcommandRouter(subcommand string) error {
	var e error
	var error_string string
	subcommand = strings.ToUpper(subcommand)
	switch subcommand {
	case "GET":
		// TODO
		if len(os.Args) == 3 {
			r := regexp.MustCompile(`^\/.*[^\/]$`)
			rs := r.FindString(os.Args[2])
			if rs == os.Args[2] {
				getData(os.Args[2])
			} else { // TODO error
				error_string = badSubcommand(fmt.Sprintf("Unrecognised primary key %s to %s.\nIt must begin with a / and not end with a /.", os.Args[2], subcommand)) // TODO DRY
			}
		} else {
			error_string = badSubcommand(fmt.Sprintf("Unrecognised %s syntax.", subcommand)) // TODO DRY
		}
	case "POST":
		// TODO
		if len(os.Args) == 4 {
			r := regexp.MustCompile(`^\/.*[^\/]$`) // this is used for path filtering
			rs := r.FindString(os.Args[2])
			if rs == os.Args[2] {
				//postData(os.Args[2], os.Args[3])
				// TODO upload files or strings
			} else { // TODO error
				error_string = badSubcommand(fmt.Sprintf("Unrecognised primary key %s to %s.\nIt must begin with a / and not end with a /.", os.Args[2], subcommand)) // TODO DRY
			}
		} else {
			error_string = badSubcommand(fmt.Sprintf("Unrecognised %s syntax.", subcommand)) // TODO DRY
		}
	default:
		error_string = badSubcommand(fmt.Sprintf("Unrecognised subcommand %q.", os.Args[1]))
	}
	if error_string != "" {
		e = errors.New(error_string)
	}
	return e
}

func getData(key string) {
	// TODO
	host := "http://localhost" // TODO into config
	port := 5380               // TODO into config
	url := host + ":" + fmt.Sprint(port) + key
	response, err := http.Get(url)
	if err != nil {
		// TODO error handling
		fmt.Println("ERROR: ", err.Error())
	} else {
		fmt.Println(response)
	}
}

func postData(key string, value []byte) {
	// TODO
	// TODO DRY
	host := "http://localhost" // TODO into config
	port := 5380               // TODO into config
	url := host + ":" + fmt.Sprint(port) + key
	response, err := http.Post(url, "text/plain; charset=UTF-8", bytes.NewReader(value))
	if err != nil {
		// TODO error handling
		fmt.Println("ERROR: ", err.Error())
	} else {
		fmt.Println(ioutil.ReadAll(response.Body))
	}
	defer response.Body.Close()
}

func deleteData(key string) {
	// TODO
}

func putData(key, value string) {
	// TODO
}

func main() {
	fmt.Println("Starting Nabia client...")
	//getData()
	var err error
	if len(os.Args) > 1 {
		err = subcommandRouter(os.Args[1])
	} else {
		fmt.Print(badSubcommand("Not enough arguments."))
		os.Exit(1)
	}
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
}
