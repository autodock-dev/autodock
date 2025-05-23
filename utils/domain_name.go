package utils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Examples:
// api.example.com -> example.com
// example.com -> example.com
func GetRootDomain(domainName string) string {
	parts := strings.Split(domainName, ".")
	if len(parts) < 2 {
		return domainName
	}
	return parts[len(parts)-2] + "." + parts[len(parts)-1]
}

// Examples:
// example.com -> ExampleDotCom
// ielts-all-in.com -> IeltsDashAllDashInDotCom
func ToAlphabel(domainName string) string {
	parts := strings.Split(domainName, ".")
	for i, part := range parts {
		parts[i] = cases.Title(language.English).String(part)
	}
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(part, "-", "Dash")
	}

	return strings.Join(parts, "Dot")
}
