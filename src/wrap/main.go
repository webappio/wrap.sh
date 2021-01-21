package main

import (
	"encoding/json"
	"github.com/layer-devops/wrap.sh/src/protocol"
	wrap "github.com/layer-devops/wrap.sh/src/wrap/pkg"
	"github.com/pborman/getopt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var LocalDevBuild = "false"
var DebugLog = "false"

func main() {
	log.SetFlags(0)
	wsLoc := "wss://" + protocol.ServerDomain + "/" + protocol.WrapServerPath

	//noinspection GoBoolExpressions
	if LocalDevBuild == "true" {
		wsLoc = "ws://" + protocol.DevServerDomain + "/" + protocol.WrapServerPath
	}

	authTokenFlag := getopt.StringLong("token", 't', "", "Your wrap.sh authentication token")
	authFileFlag := getopt.StringLong("token-file", 'f', "", "A file containing your wrap.sh authentication token")
	settingsFileFlag := getopt.StringLong("settings", 's', "", "A JSON file containing client settings")
	retryFlag := getopt.IntLong("retry", 'r', -1, "Number of times to retry the command before failing.")
	getopt.Parse()
	testCommand := ""
	for _, arg := range getopt.Args() {
		if !strings.HasPrefix(arg, "-") {
			testCommand = arg
			break
		}
	}

	authToken := *authTokenFlag
	if authToken == "" {
		if *authFileFlag != "" {
			b, err := ioutil.ReadFile(*authFileFlag)
			if err != nil {
				log.Fatal("Could not read from the specified auth token file")
			}
			authToken = strings.TrimSpace(string(b))
			if authToken == "" {
				log.Fatal("Could not find an auth token in the specified file")
			}
		}
		if authToken == "" {
			authToken = os.Getenv("WRAPSH_AUTH_TOKEN")
		}
	}

	settings := map[string]interface{}{}
	// read the settings file if one was specified
	if *settingsFileFlag != "" {
		b, err := ioutil.ReadFile(*settingsFileFlag)
		if err != nil {
			log.Fatal("Could not read from the specified settings file!")
		}
		err = json.Unmarshal(b, &settings)
		if err != nil {
			log.Fatal("Could not parse JSON from the specified settings file!")
		}
	}

	// Pull list of redacted metadata fields from the settings
	excludedTelemetryFields := map[string]bool{}
	if d, ok := settings["ExcludedTelemetryFields"]; ok {
		fields, ok := d.([]interface{})
		if ok {
			for _, f := range fields {
				field, ok := f.(string)
				if ok {
					excludedTelemetryFields[field] = true
				}
			}
		}
	}

	// Check the settings for a test command if one wasn't specified in args
	if testCommand == "" {
		if tc, ok := settings["Run"]; ok {
			tcStr, ok := tc.(string)
			if ok {
				testCommand = tcStr
			}
		}
	}

	//noinspection GoBoolExpressions
	client := &wrap.Client{
		Token:                   authToken,
		WebsocketLocation:       wsLoc,
		LogDebug:                DebugLog == "true",
		ExcludedTelemetryFields: excludedTelemetryFields,
		TestCommand:             testCommand,
	}

	// Set an access timeout if one is specified in the settings.
	// The wrap client will shut down if not accessed within this amount of minutes
	if t, ok := settings["Timeout"]; ok {
		timeout, ok := t.(float64)
		if ok {
			client.TimeoutMinutes = int(timeout)
		}
	}

	// Check the settings for a retry policy if one wasn't specified in args
	if *retryFlag == -1 {
		if entry, ok := settings["NumRetries"]; ok {
			r, ok := entry.(float64)
			if ok {
				client.NumRetries = int(r)
			}
		}
	} else {
		client.NumRetries = *retryFlag
	}

	client.Run()
	os.Exit(client.ExitCode)
}
