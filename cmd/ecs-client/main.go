package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/in4it/ecs-deploy"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
)

type Token struct {
	Token  string `json:"token" binding:"required"`
	Expire string `json:"expire" binding:"required"`
}
type Session struct {
	Token  string
	Url    string
	Expire string
}

type LoginFlags struct {
	Url string
}
type DeployFlags struct {
	ServiceName string
	Filename    string
}

type DeployResponse struct {
	Errors   map[string]string        `json:"errors" binding:"required"`
	Failures int64                    `json:"failures" binding:"required"`
	Messages []ecsdeploy.DeployResult `json:"messages"`
}
type DeployStatusResponse struct {
	Service ecsdeploy.DeployResult `json:"service" binding:"required"`
}

func addLoginFlags(f *LoginFlags, fs *pflag.FlagSet) {
	fs.StringVar(&f.Url, "url", f.Url, "ecs-deploy url, e.g. https://127.0.0.1:8080/ecs-deploy")
}
func addDeployFlags(f *DeployFlags, fs *pflag.FlagSet) {
	fs.StringVar(&f.ServiceName, "service-name", f.ServiceName, "Service name to deploy")
	fs.StringVarP(&f.Filename, "filename", "f", f.Filename, "filename to deploy")
}

func main() {
	var err error
	session, err := readSession()
	if err != nil {
		fmt.Printf("%v", err.Error())
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "login" {
		// login
		loginFlags := &LoginFlags{}
		addLoginFlags(loginFlags, pflag.CommandLine)

		if len(os.Args) > 2 && os.Args[2] != "" {
			pflag.CommandLine.Parse(os.Args[2:])
			if loginFlags.Url == "" {
				fmt.Fprintf(os.Stderr, "Usage of %s login:\n", os.Args[0])
				pflag.PrintDefaults()
				os.Exit(1)
			}
			err = login(loginFlags)
		} else {
			fmt.Fprintf(os.Stderr, "Usage of %s login:\n", os.Args[0])
			pflag.PrintDefaults()
		}
	} else if len(os.Args) > 2 && os.Args[1] == "createrepo" && os.Args[2] != "" {
		// create repo
		var result string
		result, err = createRepository(session, os.Args[2])
		fmt.Printf("%v\n", result)
	} else if len(os.Args) > 1 && os.Args[1] == "deploy" {
		// deploy
		deployFlags := &DeployFlags{}
		addDeployFlags(deployFlags, pflag.CommandLine)

		if len(os.Args) > 2 && os.Args[2] != "" {
			pflag.CommandLine.Parse(os.Args[2:])
			failure, err := deploy(session, deployFlags)
			if failure {
				if err != nil {
					fmt.Printf("%v", err.Error())
				}
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Usage of %s deploy:\n", os.Args[0])
			pflag.PrintDefaults()
		}
	} else {
		fmt.Println("Usage: ")
		fmt.Printf("%v login        login\n", os.Args[0])
		fmt.Printf("%v createrepo   create repository\n", os.Args[0])
		fmt.Printf("%v deploy       deploy services\n", os.Args[0])
	}
	if err != nil {
		fmt.Printf("%v", err.Error())
		os.Exit(1)
	}
}

// deploy with timeouts
// if --service-name is set, look for ecs.json and ecs.*.json (if filename is set, use it as directory to look into)
// if --service-name is set, with filename, give error
// if filename is set but not service name, expect serviceName in json (normal behavior)

func deploy(session Session, deployFlags *DeployFlags) (bool, error) {
	deployData, err := getDeployData(session, deployFlags)
	if err != nil {
		return true, err
	}
	response, err := doDeployAPICall(session, deployData)
	if err != nil {
		return true, err
	}
	deployed, err := waitForDeploy(session, response)
	if err != nil {
		return true, err
	}
	fmt.Println("")
	fmt.Println("---")
	var failure bool
	for k, status := range deployed {
		fmt.Printf("Service %v deployment status: %v\n", k, status)
		if status != "success" {
			failure = true
		}
	}
	return failure, nil
}
func waitForDeploy(session Session, response []byte) (map[string]string, error) {
	// api call returned info to follow-up on deployment
	var deploymentsFinished bool
	var deployResponse DeployResponse
	var finished int64
	deployed := make(map[string]string)
	maxWait := 1200
	err := json.Unmarshal(response, &deployResponse)
	if err != nil {
		return deployed, err
	}
	for _, v := range deployResponse.Messages {
		deployed[v.ServiceName] = "running"
	}
	finished += deployResponse.Failures
	for k, v := range deployResponse.Errors {
		fmt.Printf("Service %v: %v", k, v)
		deployed[k] = "error"
	}
	if int64(len(deployed)) == finished {
		deploymentsFinished = true
	}
	for i := 0; i < (maxWait/15) && !deploymentsFinished; i++ {
		for _, v := range deployResponse.Messages {
			status, err := checkDeployStatus(session, v.ServiceName, v.DeploymentTime.Format("2006-01-02T15:04:05.999999999Z"))
			if err != nil {
				return deployed, err
			}
			fmt.Printf(".")
			if status != "running" {
				deployed[v.ServiceName] = status
				fmt.Printf("%v=%v", v.ServiceName, status)
				finished++
			}
		}
		if int64(len(deployed)) == finished {
			deploymentsFinished = true
		}
		time.Sleep(15 * time.Second)
	}
	return deployed, nil
}
func checkDeployStatus(session Session, serviceName, deploymentTime string) (string, error) {
	var status string
	var deployStatusResponse DeployStatusResponse
	req, err := http.NewRequest("GET", session.Url+"/api/v1/deploy/status/"+serviceName+"/"+deploymentTime, nil)
	if err != nil {
		return status, err
	}
	req.Header.Set("Authorization", "Bearer "+session.Token)
	var client = &http.Client{
		Timeout: time.Second * 15,
	}
	resp, err := client.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 {
			return status, fmt.Errorf("Invalid credentials: use %v login --url <url> to login again\n", os.Args[0])
		} else {
			return status, fmt.Errorf("Error %d: %v", resp.StatusCode, string(body))
		}
	}
	err = json.Unmarshal(body, &deployStatusResponse)
	if err != nil {
		return status, err
	}
	status = deployStatusResponse.Service.Status
	return status, nil
}
func doDeployAPICall(session Session, deployData string) ([]byte, error) {
	var body []byte
	req, err := http.NewRequest("POST", session.Url+"/api/v1/deploy", bytes.NewBuffer([]byte(deployData)))
	if err != nil {
		return body, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.Token)
	var client = &http.Client{
		Timeout: time.Second * 120,
	}
	resp, err := client.Do(req)
	if err != nil {
		return body, err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 {
			return body, fmt.Errorf("Invalid credentials: use %v login --url <url> to login again\n", os.Args[0])
		} else {
			return body, fmt.Errorf("Error %d: %v", resp.StatusCode, string(body))
		}
	}
	return body, nil
}
func getDeployData(session Session, deployFlags *DeployFlags) (string, error) {
	var deployData string
	var readDir string
	controller := ecsdeploy.Controller{}
	if deployFlags.ServiceName != "" {
		// servicename is set
		if deployFlags.Filename != "" {
			if ok, _ := isDir(deployFlags.Filename); !ok {
				return deployData, fmt.Errorf("%v needs to be a directory if --service-name is specified\n", deployFlags.Filename)
			}
			readDir = deployFlags.Filename
		} else {
			readDir = "./"
		}
		files, err := ioutil.ReadDir(readDir)
		if err != nil {
			return deployData, err
		}
		fs := []string{}
		for _, f := range files {
			if f.Name() == "ecs.json" {
				fs = append(fs, f.Name())
			} else if strings.HasPrefix(f.Name(), "ecs.") && strings.HasSuffix(f.Name(), ".json") {
				fs = append(fs, f.Name())
			}
		}
		var deployServices ecsdeploy.DeployServices
		for _, f := range fs {
			var deploy ecsdeploy.Deploy
			controller.SetDeployDefaults(&deploy)

			content, err := ioutil.ReadFile(filepath.Join(readDir, f))
			if err != nil {
				return deployData, fmt.Errorf("Could not read file: %v\n", filepath.Join(readDir, f))
			}
			err = json.Unmarshal(content, &deploy)
			if err != nil {
				return deployData, fmt.Errorf("file %v in wrong format: %v", f, err.Error())
			}
			if f == "ecs.json" {
				deploy.ServiceName = deployFlags.ServiceName
			} else {
				start := 4
				end := len(f) - 5
				deploy.ServiceName = deployFlags.ServiceName + "-" + f[start:end]
			}
			deployServices.Services = append(deployServices.Services, deploy)
		}
		b, err := json.Marshal(deployServices)
		if err != nil {
			return deployData, err
		}
		deployData = string(b)
	} else if deployFlags.ServiceName == "" && deployFlags.Filename != "" {
		// serviceName is not set
		content, err := ioutil.ReadFile(deployFlags.Filename)
		if ok, _ := isDir(deployFlags.Filename); ok {
			return deployData, fmt.Errorf("%v is a directory. Specify a file or use the --service-name argument\n", deployFlags.Filename)
		}
		if err != nil {
			return deployData, fmt.Errorf("Could not read file: %v", deployFlags.Filename)
		}
		deployData = string(content)
	} else {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
		return deployData, errors.New("InvalidFlags")
	}
	if deployData == `{"deploy":null}` {
		return deployData, errors.New("No deployment data found")
	}
	return deployData, nil
}

func createRepository(session Session, repository string) (string, error) {
	var res string
	req, err := http.NewRequest("POST", session.Url+"/api/v1/ecr/create/"+repository, bytes.NewBuffer([]byte("")))
	if err != nil {
		return res, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.Token)
	var client = &http.Client{
		Timeout: time.Second * 60,
	}
	resp, err := client.Do(req)
	if err != nil {
		return res, err
	}
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 {
			return res, fmt.Errorf("Invalid credentials: use %v login --url <url> to login again\n", os.Args[0])
		} else {
			return res, fmt.Errorf("ecr create return http error %d", resp.StatusCode)
		}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, err
	}
	res = string(body)
	return res, nil
}

func readSession() (Session, error) {
	var session Session
	content, err := ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), ".ecsdeploy", "session.json"))
	if err != nil {
		return session, err
	}
	err = json.Unmarshal(content, &session)
	if err != nil {
		return session, err
	}
	return session, nil

}
func login(loginFlags *LoginFlags) error {
	var session Session
	var err error
	var username, password string

	session.Url = loginFlags.Url
	if os.Getenv("ECS_DEPLOY_LOGIN") != "" && os.Getenv("ECS_DEPLOY_PASSWORD") != "" {
		username = os.Getenv("ECS_DEPLOY_LOGIN")
		password = os.Getenv("ECS_DEPLOY_PASSWORD")
	} else {
		username, password, err = readCredentials()
		if err != nil {
			return err
		}
	}
	token, err := auth(session.Url, username, password)
	if err != nil {
		return err
	}
	newpath := filepath.Join(os.Getenv("HOME"), ".ecsdeploy")
	os.MkdirAll(newpath, os.ModePerm)

	session.Token = token.Token
	session.Expire = token.Expire

	b, err := json.Marshal(session)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(os.Getenv("HOME"), ".ecsdeploy", "session.json"), b, 0600)
	if err != nil {
		return err
	}
	fmt.Println("Authentication successful")
	return nil
}
func readCredentials() (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: ")
	username, _ := reader.ReadString('\n')

	fmt.Print("Enter Password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}
	password := string(bytePassword)

	return strings.TrimSpace(username), strings.TrimSpace(password), nil
}

func auth(url, login, password string) (Token, error) {
	var token Token
	var jsonStr = []byte("{\"username\":\"" + login + "\",\"password\":\"" + password + "\"}")
	req, err := http.NewRequest("POST", url+"/login", bytes.NewBuffer(jsonStr))
	if err != nil {
		return token, err
	}
	req.Header.Set("Content-Type", "application/json")
	var client = &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		return token, err
	}
	if resp.StatusCode != 200 {
		return token, errors.New("Authentication failed")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return token, err
	}

	err = json.Unmarshal(body, &token)
	if err != nil {
		return token, err
	}

	return token, nil
}
func isDir(pth string) (bool, error) {
	fi, err := os.Stat(pth)
	if err != nil {
		return false, err
	}

	return fi.Mode().IsDir(), nil
}
