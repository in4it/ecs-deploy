package util

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// askForConfirmation asks the user for confirmation. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user.
func AskForConfirmation(s string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}

// stackoverflow
var randSrc = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A randSrc.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func YesNoToBool(s string) bool {
	if strings.ToLower(s) == "yes" {
		return true
	} else {
		return false
	}
}

func TruncateString(str string, n int) string {
	if n > 0 && len(str) > n {
		return str[0:n]
	} else {
		return str
	}
}

/*
 * RemoveCommonElements removes the common elements and returns the first array
 */
func RemoveCommonElements(a, b []string) []string {
	var c = []string{}
	var d = []string{}

	for _, item1 := range a {
		for _, item2 := range b {
			if item1 == item2 {
				c = append(c, item2)
			}
		}
	}

	for _, item1 := range a {
		found := false
		for _, item2 := range c {
			if item1 == item2 {
				found = true
			}
		}
		if !found {
			d = append(d, item1)
		}
	}
	return d
}

/*
 * IsBoolArrayTrue checks whether the array contains only true elements
 */
func IsBoolArrayTrue(array []bool) bool {
	if len(array) == 0 {
		return false
	}
	for _, v := range array {
		if !v {
			return false
		}
	}
	return true
}

/*
 * InArray returns true if the value exists in the array
 */
func InArray(a []string, v string) (ret bool, i int) {
	for i = range a {
		if ret = a[i] == v; ret {
			return ret, i
		}
	}
	return false, -1
}
