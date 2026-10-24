package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

// InstallCliFromNpmInput is the request to install a CLI plugin from a published npm package.
type InstallCliFromNpmInput struct {
	NpmPackage string `json:"npm_package"`
	Name       string `json:"name"`
}

// InstallCliFromNpmOutput returns the persisted ExtensionItem after a successful install.
type InstallCliFromNpmOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
