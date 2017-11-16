package main

import (
  "github.com/juju/loggo"

  "os"
  "fmt"
)

func getEnv(key, fallback string) string {
  if value, ok := os.LookupEnv(key); ok {
      return value
  }
  return fallback
}

func envExists(key string) bool {
  _, ok := os.LookupEnv(key)
  return ok
}

func startup_checks() {
  mandatoryEnvVars := []string{
    "AWS_REGION",
    "JWT_SECRET",
    "DEPLOY_PASSWORD",
    "DEVELOPER_PASSWORD",
  }
  for _, envVar := range mandatoryEnvVars {
    if ! envExists(envVar) {
      fmt.Printf("Environment variable missing: %v\n", envVar)
      os.Exit(1)
    }
  }
}

func main() {
  // set logging to debug
  if getEnv("DEBUG", "") == "true" {
    loggo.ConfigureLoggers(`<root>=DEBUG`)
  }

  // startup checks
  startup_checks();

  // Launch API
  api := API{}
  api.launch()
}

