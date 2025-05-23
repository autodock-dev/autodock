package utils

import "testing"

func TestGetRootDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api.example.com", "example.com"},
		{"example.com", "example.com"},
		{"foo.bar.example.com", "example.com"},
		{"localhost", "localhost"},
		{"", ""},
		{"co.uk", "co.uk"},
	}

	for _, test := range tests {
		result := GetRootDomain(test.input)
		if result != test.expected {
			t.Errorf("GetRootDomain(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestToAlphabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api.example.com", "ApiDotExampleDotCom"},
		{"example.com", "ExampleDotCom"},
		{"foo.bar.example.com", "FooDotBarDotExampleDotCom"},
		{"localhost", "Localhost"},
		{"", ""},
		{"co.uk", "CoDotUk"},
		{"ielts-all-in.com", "IeltsDashAllDashInDotCom"},
	}

	for _, test := range tests {
		result := ToAlphabel(test.input)
		if result != test.expected {
			t.Errorf("ToAlphabel(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}
