package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func detectFileMimetype(filename string) string {
	mtype, err := mimetype.DetectFile(filename)
	if err != nil {
		log.Fatalf("ERROR detecting filetype: %q", err)
	}
	return mtype.String()
}

func detectBytesliceMimetype(byteSlice []byte) string {
	mtype := mimetype.Detect(byteSlice)
	return mtype.String()
}

func makeRequest(method string, key string, host string, port uint16, value []byte, ctype ...string) (*http.Response, error) {
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.Itoa(int(port))),
		Path:   key,
	}

	var req *http.Request
	var err error

	if value != nil {
		req, err = http.NewRequest(method, u.String(), bytes.NewReader(value))
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}

	if err != nil {
		return nil, err
	}

	if len(ctype) == 0 { // unknown Content-Type, let's set a default
		req.Header.Set("Content-Type", "application/octet-stream") // https://www.iana.org/assignments/media-types/application/octet-stream
	} else { // Content-Type was set
		req.Header.Set("Content-Type", ctype[0]) // https://www.iana.org/assignments/media-types/application/octet-stream
	}
	req.Header.Set("User-Agent", "nabia-client/0.1")

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func optionsData(key string, host string, port uint16) (string, error) {
	response, err := makeRequest("OPTIONS", key, host, port, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	optionsString := response.Header.Get("Allow")
	if len(optionsString) == 0 {
		log.Fatalf("Unknown error while trying to \"OPTIONS\" %s at %s:%d: Empty Allow header", key, host, port)
	}
	return optionsString, nil
}

func headData(key string, host string, port uint16) (bool, error) {
	response, err := makeRequest("HEAD", key, host, port, nil)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.StatusCode/100 != 2 {
		return false, nil
	}

	return true, nil
}

func getData(key string, host string, port uint16) ([]byte, string, error) {
	response, err := makeRequest("GET", key, host, port, nil)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	if response.StatusCode/100 != 2 {
		return nil, "", fmt.Errorf("expected 2xx response code, got %s", response.Status)
	}

	ctype := response.Header.Get("Content-Type")

	return body, ctype, nil
}

func postData(key string, host string, port uint16, value []byte, ctype string) error {
	response, err := makeRequest("POST", key, host, port, value, ctype)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode/100 != 2 {
		return fmt.Errorf("expected 2xx response code, got %s", response.Status)
	}

	return nil
}

func putData(key string, host string, port uint16, value []byte, ctype string) error {
	response, err := makeRequest("PUT", key, host, port, value, ctype)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode/100 != 2 {
		return fmt.Errorf("expected 2xx response code, got %s", response.Status)
	}

	return nil
}

func deleteData(key string, host string, port uint16) error {
	response, err := makeRequest("DELETE", key, host, port, nil)
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
			fmt.Printf("Getting key %s from %s:%d\n", key, host, port)
			data, ctype, err := getData(key, host, uint16(port))
			if err != nil {
				log.Fatalf(err.Error())
			} else {
				if ctype == "text/plain; charset=utf-8" && utf8.Valid(data) {
					fmt.Printf("%q\n", string(data))
				} else {
					fmt.Println("Data is not plaintext Unicode, refusing to print to stdout.")
				}
			}
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

			var content []byte
			var err error

			ctype := "application/octet-stream"

			if filePath != "" {
				// filePath is provided, read the file and post its content
				content, err = os.ReadFile(filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error reading file:", err)
					return
				}
				ctype = detectBytesliceMimetype(content)
				fmt.Printf("Posting content of file %s to key %s at %s:%d\n", filePath, key, host, port)
			} else if len(args) > 1 {
				// value is provided as a second argument, post it as is
				content = []byte(args[1])
				if utf8.Valid(content) {
					ctype = "text/plain; charset=utf-8"
					fmt.Printf("Posting value %q to key %s at %s:%d\n", string(content), key, host, port)
				} else {
					fmt.Println("Non-Unicode value provided as argument. To POST arbitrary bytes, please see the --file flag")
				}
			} else {
				log.Fatal("Either a value or --file must be provided")
			}
			ctype = detectBytesliceMimetype(content)
			err = postData(key, host, uint16(port), content, ctype)
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

			var content []byte
			var err error

			ctype := "application/octet-stream"

			if filePath != "" {
				// filePath is provided, read the file and put its content
				content, err = os.ReadFile(filePath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error reading file:", err)
					return
				}
				fmt.Printf("Putting content of file %s to key %s at %s:%d\n", filePath, key, host, port)
			} else if len(args) > 1 {
				// value is provided as a second argument, put it as is
				content = []byte(args[1])
				if utf8.Valid(content) {
					ctype = "text/plain; charset=utf-8"
					fmt.Printf("Putting value %q to key %s at %s:%d\n", string(content), key, host, port)
				} else {
					fmt.Println("Non-Unicode value provided as argument. To POST arbitrary bytes, please see the --file flag")
				}
			} else {
				log.Fatal("Either a value or --file must be provided")
			}
			ctype = detectBytesliceMimetype(content)
			err = putData(key, host, uint16(port), content, ctype)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		},
	}

	var deleteCmd = &cobra.Command{
		Use:   "DELETE [key]",
		Short: "DELETE a key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")

			fmt.Printf("Deleting key %s from %s:%d\n", key, host, port)
			err := deleteData(key, host, uint16(port))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		},
	}

	var headCmd = &cobra.Command{
		Use:   "HEAD [key]",
		Short: "HEAD (check if exists) key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")

			fmt.Printf("Checking if key %s exists at %s:%d\n", key, host, port)
			exists, err := headData(key, host, uint16(port))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else if exists {
				fmt.Printf("Key %q exists\n", key)
			} else {
				fmt.Printf("Key %q does not exist\n", key)
			}
		},
	}

	var optionsCmd = &cobra.Command{
		Use:   "OPTIONS [key]",
		Short: "OPTIONS (check available methods) key",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			host := viper.GetString("host")
			port := viper.GetInt("port")

			fmt.Printf("Checking available methods for key %s at %s:%d\n", key, host, port)
			optionsString, err := optionsData(key, host, uint16(port))
			if err != nil {
				log.Fatalf("Error: %s", err)
			} else {
				fmt.Printf("%s\n", optionsString)
			}
		},
	}

	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(postCmd)
	rootCmd.AddCommand(putCmd)
	rootCmd.AddCommand(headCmd)
	rootCmd.AddCommand(optionsCmd)

	pflag.String("host", "localhost", "Nabia server host")
	pflag.Uint16("port", 5380, "Nabia server port")
	pflag.String("file", "", "Path to a file, uploaded with POST or PUT, and downloaded with GET")
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
