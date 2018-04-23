package fckrkn

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func Split(s string, re *regexp.Regexp) []string {
	var (
		r []string
		p int
	)
	is := re.FindAllStringIndex(s, -1)
	if is == nil {
		return append(r, s)
	}
	for _, i := range is {
		r = append(r, s[p:i[0]])
		p = i[1]
	}
	return append(r, s[p:])
}

type ParseProxyErr struct {
	message string
}

func (e ParseProxyErr) Error() string {
	return e.message
}

type ParseCredentialsErr struct {
	message string
}

func (e ParseCredentialsErr) Error() string {
	return e.message
}

type ParseHostErr struct {
	message string
}

func (e ParseHostErr) Error() string {
	return e.message
}

func ParseProxy(proxyString string) (proxyType, proxyHost, proxyPort, proxyUser, proxyPass string, err error) {
	segments := Split(proxyString, reProxy)
	var hostIdx int

	switch len(segments) {
	case 3:
		credentials := strings.Split(segments[1], ":")
		hostIdx = 2
		if len(credentials) == 1 {
			err = ParseCredentialsErr{"Can't parse credentials"}
		} else if len(credentials) == 2 {
			proxyUser = credentials[0]
			proxyPass = credentials[1]
		}
	case 2:
		hostIdx = 1
	default:
		err = ParseProxyErr{"Can't parse proxy string"}
	}

	proxyType = segments[0]
	elements := strings.Split(segments[hostIdx], ":")
	if len(elements) == 1 {
		err = ParseHostErr{"Can't parse host"}
	} else if len(elements) == 2 {
		proxyHost = elements[0]
		proxyPort = elements[1]
	}

	return
}

type CheckProxyErr struct {
	message string
}

func (e CheckProxyErr) Error() string {
	return e.message
}

func CheckProxy(proxyString string, opts *Options) (error, bool) {
	proxyURL, err := url.Parse(proxyString)
	if err != nil {
		return err, false
	}
	transport := http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	client := http.Client{
		Transport: &transport,
		Timeout:   time.Duration(time.Second * 1),
	}
	resp, err := client.Get(fmt.Sprintf("%v:%v", opts.host, opts.port))
	if err != nil {
		return err, false
	}

	Verbose.Printf("Proxy: %s\n\tStatus code: %d", proxyString, resp.StatusCode)
	if resp.StatusCode >= 300 {
		return CheckProxyErr{
			fmt.Sprintf("Proxy verification failed. Response status code: %d", resp.StatusCode),
		}, false
	}

	return nil, true
}

func GetSetupProxyURL(proxyString string) (string, error) {
	_, proxyHost, proxyPort, proxyUser, proxyPass, err := ParseProxy(proxyString)
	if err != nil {
		return "", err
	}

	setupURL := "https://t.me/socks?"

	if proxyUser != "" {
		setupURL += fmt.Sprintf(
			"server=%s&port=%s&user=%s&pass=%s",
			proxyHost, proxyPort, proxyUser, proxyPass,
		)
	} else {
		setupURL += fmt.Sprintf("server=%s&port=%s", proxyHost, proxyPort)
	}

	return setupURL, nil
}
