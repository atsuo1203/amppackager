// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TODO(twifkak): Test this.
// TODO(twifkak): Document code.
package main

import (
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/nyaxt/webpackage/go/signedexchange"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	amppkg "github.com/ampproject/amppackager"
)

var flagConfig = flag.String("config", "amppkg.toml", "Path to the config toml file.")

type Config struct {
	LocalOnly    bool
	Port         int
	PackagerBase string // The base URL under which /amppkg/ URLs will be served on the internet.
	CertFile     string // This must be the full certificate chain.
	KeyFile      string // Just for the first cert, obviously.
	GoogleAPIKey string
	URLSet       []amppkg.URLSet
}

var dotStarRegexp = ".*"

// Also sets defaults.
func validateURLPattern(pattern *amppkg.URLPattern, allowedSchemes map[string]bool) error {
	if len(pattern.Scheme) == 0 {
		// Default Scheme to the list of keys in allowedSchemes.
		pattern.Scheme = make([]string, len(allowedSchemes))
		i := 0
		for scheme := range allowedSchemes {
			pattern.Scheme[i] = scheme
			i++
		}
	} else {
		for _, scheme := range pattern.Scheme {
			if !allowedSchemes[scheme] {
				return errors.Errorf("Scheme contains invalid value %q", scheme)
			}
		}
	}
	if pattern.Domain == "" {
		return errors.New("Domain must be specified")
	}
	if pattern.PathRE == nil {
		pattern.PathRE = &dotStarRegexp
	} else if _, err := regexp.Compile(*pattern.PathRE); err != nil {
		return errors.New("PathRE must be a valid regexp")
	}
	for _, exclude := range pattern.PathExcludeRE {
		if _, err := regexp.Compile(exclude); err != nil {
			return errors.Errorf("PathExcludeRE contains be invalid regexp %q", exclude)
		}
	}
	if pattern.QueryRE == nil {
		pattern.QueryRE = &dotStarRegexp
	} else if _, err := regexp.Compile(*pattern.QueryRE); err != nil {
		return errors.New("QueryRE must be a valid regexp")
	}
	return nil
}

var allowedFetchSchemes = map[string]bool{"http": true, "https": true}
var allowedSignSchemes = map[string]bool{"https": true}

// Reads the config file specified at --config and validates it.
func readConfig() (*Config, error) {
	if *flagConfig == "" {
		return nil, errors.New("must specify --config")
	}
	tree, err := toml.LoadFile(*flagConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse config at: %s", *flagConfig)
	}
	config := Config{}
	if err = tree.Unmarshal(&config); err != nil {
		return nil, errors.Wrapf(err, "failed to parse config at: %s", *flagConfig)
	}
	// TODO(twifkak): Return an error if the TOML includes any fields that aren't part of the Config struct.

	if config.Port == 0 {
		config.Port = 8080
	}
	if !strings.HasSuffix(config.PackagerBase, "/") {
		// This ensures that the ResolveReference call doesn't replace the last path component.
		config.PackagerBase += "/"
	}
	if config.CertFile == "" {
		return nil, errors.New("must specify CertFile")
	}
	if config.KeyFile == "" {
		return nil, errors.New("must specify KeyFile")
	}
	if config.GoogleAPIKey == "" {
		return nil, errors.New("must specify GoogleAPIKey")
	}
	if len(config.URLSet) == 0 {
		return nil, errors.New("must specify one or more [[URLSet]]")
	}
	for i := range config.URLSet {
		if err := validateURLPattern(&config.URLSet[i].Fetch, allowedFetchSchemes); err != nil {
			return nil, errors.Wrapf(err, "parsing URLSet.%d.Fetch", i)
		}
		if err := validateURLPattern(&config.URLSet[i].Sign, allowedSignSchemes); err != nil {
			return nil, errors.Wrapf(err, "parsing URLSet.%d.Sign", i)
		}
		if config.URLSet[i].Sign.ErrorOnStatefulHeaders {
			return nil, errors.Errorf("URLSet.%d.Sign.ErrorOnStatefulHeaders is not allowed; perhaps you meant to put this in the Fetch section?", i)
		}
	}
	return &config, nil
}

type LogIntercept struct {
	handler http.Handler
}

func (this LogIntercept) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// TODO(twifkak): Adopt whatever the standard format is nowadays.
	log.Println("Serving", req.URL, "to", req.RemoteAddr)
	this.handler.ServeHTTP(resp, req)
	// TODO(twifkak): Get status code from resp. This requires making a ResponseWriter wrapper.
	// TODO(twifkak): Separate the typical weblog from the detailed error log.
}

// Exposes an HTTP server. Don't run this on the open internet, for at least two reasons:
//  - It exposes an API that allows people to sign any URL as any other URL.
//  - It is in cleartext.
func main() {
	flag.Parse()
	config, err := readConfig()
	if err != nil {
		panic(errors.Wrap(err, "reading config"))
	}

	// TODO(twifkak): Document what cert/key storage formats this accepts.
	certPem, err := ioutil.ReadFile(config.CertFile)
	if err != nil {
		panic(errors.Wrapf(err, "reading %s", config.CertFile))
	}
	keyPem, err := ioutil.ReadFile(config.KeyFile)
	if err != nil {
		panic(errors.Wrapf(err, "reading %s", config.KeyFile))
	}

	certs, err := signedexchange.ParseCertificates(certPem)
	if err != nil {
		panic(errors.Wrapf(err, "parsing %s", config.CertFile))
	}
	if certs == nil || len(certs) == 0 {
		panic(fmt.Sprintf("no cert found in %s", config.CertFile))
	}
	cert := certs[0]
	// TODO(twifkak): Verify that cert covers all the signing domains in the config.

	keyBlock, _ := pem.Decode(keyPem)
	if keyBlock == nil {
		panic(fmt.Sprintf("no key found in %s", config.KeyFile))
	}

	key, err := signedexchange.ParsePrivateKey(keyBlock.Bytes)
	if err != nil {
		panic(errors.Wrapf(err, "parsing %s", config.KeyFile))
	}
	// TODO(twifkak): Verify that key matches cert.

	packager, err := amppkg.NewPackager(cert, key, config.PackagerBase, config.URLSet)
	if err != nil {
		panic(errors.Wrap(err, "building packager"))
	}
	certCache, err := amppkg.NewCertCache(cert, certPem)
	if err != nil {
		panic(errors.Wrap(err, "building cert cache"))
	}

	// TODO(twifkak): Make log output configurable.
	mux := http.NewServeMux()
	mux.Handle("/priv-amppkg/doc", packager)
	mux.Handle(path.Join("/", amppkg.CertUrlPrefix)+"/", certCache)
	addr := ""
	if config.LocalOnly {
		addr = "localhost"
	}
	addr += fmt.Sprint(":", config.Port)
	server := http.Server{
		Addr: addr,
		// Don't use DefaultServeMux, per
		// https://blog.cloudflare.com/exposing-go-on-the-internet/.
		Handler:           LogIntercept{mux},
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		// If needing to stream the response, disable WriteTimeout and
		// use TimeoutHandler instead, per
		// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/.
		WriteTimeout: 60 * time.Second,
		// Needs Go 1.8.
		IdleTimeout: 120 * time.Second,
		// TODO(twifkak): Specify ErrorLog?
	}

	// TODO(twifkak): Add monitoring (e.g. per the above Cloudflare blog).

	log.Println("Serving on port", config.Port)

	// TCP keep-alive timeout on ListenAndServe is 3 minutes. To shorten,
	// follow the above Cloudflare blog.
	log.Fatal(server.ListenAndServe())
}