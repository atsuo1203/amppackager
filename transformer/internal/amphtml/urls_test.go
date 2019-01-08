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

package amphtml

import (
	"net/url"
	"testing"
)

const relativeFooURL = "/foo"

func TestToAbsoluteURL(t *testing.T) {
	rootURL := "https://www.example.com/"
	fooURL := "https://www.example.com/foo"
	barURL := "https://www.example.com/bar"
	otherURL := "http://otherdomain.com"

	tcs := []struct {
		desc             string
		input            string
		baseURL          string
		documentURL      string
		expectedAbsolute string
	}{
		{
			desc:             "Empty",
			input:            "",
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: "",
		},
		{
			desc:    "protocol relative path",
			input:   "//domain.com",
			baseURL: barURL,
			// Note that the technically correct absolute URL here would be http, but
			// we 'upgrade' protocol relative to https.
			documentURL:      "http://example.com/",
			expectedAbsolute: "https://domain.com",
		},
		{
			desc:             "unusual protocol",
			input:            "file://foo.txt",
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: "file://foo.txt",
		},
		{
			desc:             "mailto protocol",
			input:            "mailto:user@example.com",
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: "mailto:user@example.com",
		},
		{
			desc:             "valid absolute",
			input:            fooURL,
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: fooURL,
		},
		{
			desc:             "valid relative",
			input:            relativeFooURL,
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: fooURL,
		},
		{
			desc:             "relative to base URL, not document URL",
			input:            relativeFooURL,
			baseURL:          rootURL,
			documentURL:      otherURL,
			expectedAbsolute: fooURL,
		},
		{
			desc:             "absolute with different base",
			input:            fooURL,
			baseURL:          otherURL,
			documentURL:      rootURL,
			expectedAbsolute: fooURL,
		},
		{
			desc:             "empty fragment preserved",
			input:            "#",
			baseURL:          rootURL,
			documentURL:      rootURL,
			expectedAbsolute: "#",
		},
		{
			desc:             "fragment same base",
			input:            barURL + "#dogs",
			baseURL:          barURL,
			documentURL:      rootURL,
			expectedAbsolute: barURL + "#dogs",
		},
		{
			desc:             "fragment different base",
			input:            barURL + "#dogs",
			baseURL:          otherURL,
			documentURL:      rootURL,
			expectedAbsolute: barURL + "#dogs",
		},
		{
			desc:             "same url ignoring fragment",
			input:            "#dogs",
			baseURL:          rootURL,
			documentURL:      rootURL,
			expectedAbsolute: "#dogs",
		},
		{
			desc:             "fragment differs from document when relative to base",
			input:            "#dogs",
			baseURL:          rootURL,
			documentURL:      otherURL,
			expectedAbsolute: rootURL + "#dogs",
		},
	}
	for _, tc := range tcs {
		baseURL, _ := url.Parse(tc.baseURL)
		actual := ToAbsoluteURL(&tc.documentURL, baseURL, &tc.input)
		if actual != tc.expectedAbsolute {
			t.Errorf("%s: ToAbsoluteURL=%s want=%s", tc.desc, actual, tc.expectedAbsolute)
		}
	}
}

func TestGetCacheURL(t *testing.T) {
	tcs := []struct {
		desc, input, expectedImage, expectedOther string
		width                                     int
		expectError                               bool
	}{
		{
			desc:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			desc:          "image",
			input:         "http://www.example.com/blah.jpg",
			expectedImage: "https://www-example-com.cdn.ampproject.org/i/www.example.com/blah.jpg",
			expectedOther: "https://www-example-com.cdn.ampproject.org/r/www.example.com/blah.jpg",
		},
		{
			desc:          "secure",
			input:         "https://www.example.com/blah.jpg",
			expectedImage: "https://www-example-com.cdn.ampproject.org/i/s/www.example.com/blah.jpg",
			expectedOther: "https://www-example-com.cdn.ampproject.org/r/s/www.example.com/blah.jpg",
		},
		{
			desc:          "image with requested width",
			input:         "http://www.example.com/blah.jpg",
			width:         50,
			expectedImage: "https://www-example-com.cdn.ampproject.org/ii/w50/www.example.com/blah.jpg",
			expectedOther: "https://www-example-com.cdn.ampproject.org/r/www.example.com/blah.jpg",
		},
		{
			desc:          "image negative width",
			input:         "http://www.example.com/blah.jpg",
			width:         -50,
			expectedImage: "https://www-example-com.cdn.ampproject.org/i/www.example.com/blah.jpg",
			expectedOther: "https://www-example-com.cdn.ampproject.org/r/www.example.com/blah.jpg",
		},
		{
			desc:          "fragment",
			input:         "https://localhost.test/icons/below.svg#icon-whatsapp",
			expectedImage: "https://localhost-test.cdn.ampproject.org/i/s/localhost.test/icons/below.svg#icon-whatsapp",
			expectedOther: "https://localhost-test.cdn.ampproject.org/r/s/localhost.test/icons/below.svg#icon-whatsapp",
		},
		{
			desc:          "port is dropped",
			input:         "http://www.example.com:8080/blah.jpg",
			expectedImage: "https://www-example-com.cdn.ampproject.org/i/www.example.com/blah.jpg",
			expectedOther: "https://www-example-com.cdn.ampproject.org/r/www.example.com/blah.jpg",
		},
		{
			desc:        "unsupported scheme noop",
			input:       "data:image/png.foo",
			expectError: true,
		},
		{
			desc:          "relative url with width",
			input:         "itshappening.gif",
			expectedImage: "https://example-com.cdn.ampproject.org/ii/w100/s/example.com/itshappening.gif",
			expectedOther: "https://example-com.cdn.ampproject.org/r/s/example.com/itshappening.gif",
			width:         100,
		},
	}
	base, _ := url.Parse("https://example.com/")
	for _, tc := range tcs {
		for _, subtype := range []SubresourceType{OtherType, ImageType} {
			expected := tc.expectedOther
			if subtype == ImageType {
				expected = tc.expectedImage
			}
			so := SubresourceOffset{SubType: subtype, Start: 0, End: len(tc.input), DesiredImageWidth: tc.width}
			rootURL := "https://example.com/"
			cu, err := so.GetCacheURL(&rootURL, base, &tc.input)
			if tc.expectError {
				if err == nil {
					t.Errorf("%s: ToCacheImageURL(%s, %d) expected error. Got none", tc.desc, tc.input, tc.width)
				}
			} else if cu.String() != expected {
				t.Errorf("%s: ToCacheImageURL(%s, %d)=%s, want=%s", tc.desc, tc.input, tc.width, cu.String(), expected)
			}
		}
	}
}

func TestToCacheURLDomain(t *testing.T) {
	tcs := []struct {
		desc, input, expected string
	}{
		{
			desc:     "simple case",
			input:    "example.com",
			expected: "example-com",
		},
		{
			desc:     "simple case, case-insensitive",
			input:    "ExAMpLE.Com",
			expected: "example-com",
		},
		{
			desc:     "origin has no dots or hyphes, use hash",
			input:    "toplevelnohyphens",
			expected: "qsgpfjzulvuaxb66z77vlhb5gu2irvcnyp6t67cz6tqo5ae6fysa",
		},
		{
			desc:     "Human-readable form too long; use hash",
			input:    "itwasadarkandstormynight.therainfellintorrents.exceptatoccasionalintervalswhenitwascheckedby.aviolentgustofwindwhichsweptupthestreets.com",
			expected: "dgz4cnrxufaulnwku4ow5biptyqnenjievjht56hd7wqinbdbteq",
		},
		{
			desc:     "IDN",
			input:    "xn--bcher-kva.ch",
			expected: "xn--bcher-ch-65a",
		},
		{
			desc:     "RTL",
			input:    "xn--4gbrim.xn----rmckbbajlc6dj7bxne2c.xn--wgbh1c",
			expected: "xn-------i5fvcbaopc6fkc0de0d9jybegt6cd",
		},
		{
			desc:     "Mixed Bidi, use hash",
			input:    "hello.xn--4gbrim.xn----rmckbbajlc6dj7bxne2c.xn--wgbh1c",
			expected: "a6h5moukddengbsjm77rvbosevwuduec2blkjva4223o4bgafgla",
		},
		{
			desc:     "Punify(ز۰.ز٠) = xn--xgb49a.xn--xgb6g. Cannot mix two alternative Arabic ranges. Use hash",
			input:    "xn--xgb49a.xn--xgb6g",
			expected: "asdk26k2mfqxgc6cdx3oh3vlnx42rqwn6uvsuqrufnx622tguq6q",
		},
		{
			desc:     "Same Arabic range is ok",
			input:    "xn--xgb49a.xn--xgb49a",
			expected: "xn----lncb27eca",
		},
		{
			desc:     "R-LDH: cannot contain double hyphen in 3 and 4th char positions",
			input:    "in--trouble.com",
			expected: "r5s7rxu53tjelpr7ngbxkxpirbrylvbwcuueckh7gmn5mim5cjna",
		},
		{
			desc:     "R-LDH #2",
			input:    "in-trouble.com",
			expected: "j7pweznglei73fva3bo6oidjt74j3hx4tfyncjsdwud7r7cci4va",
		},
		{
			desc:     "R-LDH #3",
			input:    "a--problem.com",
			expected: "a47psvede4jpgjom2kzmuhop74zzmdpjzasoctyoqqaxbkdbsyiq",
		},
		{
			desc:     "Transition mapping per UTS #46",
			input:    "faß.de",
			expected: "fass-de",
		},
	}
	for _, tc := range tcs {
		actual := ToCacheURLSubdomain(tc.input)
		if actual != tc.expected {
			t.Errorf("%s: ToCacheURLDomain(%s)=%s, want=%s", tc.desc, tc.input, actual, tc.expected)
		}
	}
}
