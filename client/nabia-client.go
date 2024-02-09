package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func getData(key string, host string, port uint16) (interface{}, error) {
	// TODO don't assume charset
	// Constructing a structured URL
	u := url.URL{
		Scheme: "http", // or "https" for secure connections
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   key,
	}

	// Initialize a new request with http.NewRequest
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Setting headers
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", "nabia-client/0.1")

	// Send the request using http.Client
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func postData(key string, host string, port uint16, value []byte) error {
	// TODO don't assume charset
	// Constructing the URL as previously
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(int(port))),
		Path:   key,
	}

	// Creating a new request with the correct method and body
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(value))
	if err != nil {
		return err
	}

	// Setting headers
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", "nabia-client/0.1")

	// Making the request
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode/100 != 2 {
		return fmt.Errorf("expected 2xx response code, got %s", response.Status)
	}

	return nil
}

func deleteData(key string) {
	// TODO
}

func putData(key string, host string, port uint16, value []byte) error {
	// TODO DRY
	// TODO don't assume charset
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(int(port))),
		Path:   key,
	}
	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewReader(value))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=UTF-8")
	req.Header.Set("User-Agent", "nabia-client/0.1")

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode/100 != 2 {
		return fmt.Errorf("expected 2xx response code, got %s", response.Status)
	}

	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "nabia-client",
		Short: "Nabia client application",
	}

	var getCmd = &cobra.Command{
		Use:   "GET [key]",
		Short: "GET a key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")

			// Your code to get the key goes here
			fmt.Printf("Getting key %s from %s:%d\n", key, host, port)
			getData(key, host, uint16(port))
		},
	}

	var postCmd = &cobra.Command{
		Use:   "POST [key] [value]",
		Short: "POST value to a key",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")
			filePath, _ := cmd.Flags().GetString("file")

			var value string
			var err error

			if filePath != "" {
				// filePath is provided, read the file and post its content
				content, err := os.ReadFile(filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error reading file:", err)
					return
				}
				value = string(content)
				fmt.Printf("Posting content of file %s to key %s at %s:%d\n", filePath, key, host, port)
			} else if len(args) > 1 {
				// value is provided as a second argument, post it as is
				value = args[1]
				fmt.Printf("Posting value %s to key %s at %s:%d\n", value, key, host, port)
			} else {
				fmt.Fprintln(os.Stderr, "Either a value or --file must be provided")
				return
			}

			err = postData(key, host, uint16(port), []byte(value))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		},
	}

	var putCmd = &cobra.Command{
		Use:   "PUT [key] [value]",
		Short: "PUT value to a key",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")
			filePath, _ := cmd.Flags().GetString("file")

			var value string
			var err error

			if filePath != "" {
				// filePath is provided, read the file and put its content
				content, err := os.ReadFile(filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error reading file:", err)
					return
				}
				value = string(content)
				fmt.Printf("Putting content of file %s to key %s at %s:%d\n", filePath, key, host, port)
			} else if len(args) > 1 {
				// value is provided as a second argument, put it as is
				value = args[1]
				fmt.Printf("Putting value %s to key %s at %s:%d\n", value, key, host, port)
			} else {
				fmt.Fprintln(os.Stderr, "Either a value or --file must be provided")
				return
			}

			err = putData(key, host, uint16(port), []byte(value))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		},
	}

	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(putCmd)

	pflag.String("host", "http://localhost", "Nabia server host")
	pflag.Uint16("port", 5380, "Nabia server port")
	pflag.String("file", "", "Path to a file to be posted")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	// TODO check the arguments

	viper.SetEnvPrefix("nabia")
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
