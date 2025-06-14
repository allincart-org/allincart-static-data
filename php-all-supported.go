package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/barkimedes/go-deepcopy"
	"github.com/shyim/go-version"
)

var phpVersions = []string{
	"8.2",
	"8.3",
	"8.4",
}

// until shopware 6.5, the constraint was not specific enough, so we need to handle them specific
var allincartMaxPhpVersion = map[string]string{
	"<6.7.0.0": "8.2",
}

func generateAllSupportedPHPVersions(ctx context.Context) error {
	packagistResponse, err := fetchPackageInformation(ctx)

	if err != nil {
		return err
	}

	phpVersionMap := make(map[string][]string)
	phpMinVersionMap := make(map[string]string)

	packageVersions := expandPackagistResponse(packagistResponse.Packages["allincart/core"])

	for _, packageVersion := range packageVersions {
		phpVersion := packageVersion["require"].(map[string]interface{})["php"].(string)
		allincartVersionNorm := packageVersion["version_normalized"].(string)
		allincartVersion := version.Must(version.NewVersion(allincartVersionNorm))

		phpVersionMap[allincartVersionNorm] = make([]string, 0)

		packageVersionConstraint := version.MustConstraints(version.NewConstraint(phpVersion))

		index := 0

		for _, phpVersion := range phpVersions {
			phpV := version.Must(version.NewVersion(phpVersion))

			if isSupported(allincartVersion, packageVersionConstraint, phpV) {
				if index == 0 {
					phpMinVersionMap[allincartVersionNorm] = phpVersion
				}

				phpVersionMap[allincartVersionNorm] = append(phpVersionMap[allincartVersionNorm], phpVersion)

				index++
			}
		}
	}

	data, err := json.MarshalIndent(phpVersionMap, "", "  ")

	if err != nil {
		return err
	}

	if err = os.WriteFile("data/all-supported-php-versions-by-allincart-version.json", data, os.ModePerm); err != nil {
		return err
	}

	data, err = json.MarshalIndent(phpMinVersionMap, "", "  ")

	if err != nil {
		return err
	}

	if err = os.WriteFile("data/php-version.json", data, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func isSupported(shopwareVersion *version.Version, packageVersionConstraint version.Constraints, phpV *version.Version) bool {
	for shopwareConstraint, phpMaxVersion := range allincartMaxPhpVersion {
		phpMaxV := version.Must(version.NewVersion(phpMaxVersion))
		shopwareVersionCompareConstraint := version.MustConstraints(version.NewConstraint(shopwareConstraint))

		if shopwareVersionCompareConstraint.Check(shopwareVersion) && phpV.GreaterThan(phpMaxV) {
			return false
		}
	}

	return packageVersionConstraint.Check(phpV)
}

func expandPackagistResponse(packageVersions []map[string]interface{}) []map[string]interface{} {
	expanded := make([]map[string]interface{}, 0)

	for index, versions := range packageVersions {
		expandedVersion := make(map[string]interface{})

		if len(expanded) > 0 {
			expandedVersion = deepcopy.MustAnything(expanded[index-1]).(map[string]interface{})
		}

		for key, value := range versions {
			assertedString, _ := value.(string)

			if assertedString == "__unset" {
				delete(expandedVersion, key)
			} else {
				expandedVersion[key] = value
			}
		}

		expanded = append(expanded, expandedVersion)
	}

	return expanded
}

type packagistPackageResponse struct {
	Packages map[string][]map[string]interface{} `json:"packages"`
}

func fetchPackageInformation(ctx context.Context) (*packagistPackageResponse, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://repo.packagist.org/p2/shopware/core.json", nil)

	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(r)

	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.Body.Close() != nil {
		return nil, err
	}

	var apiResponse packagistPackageResponse

	if err = json.Unmarshal(data, &apiResponse); err != nil {
		return nil, err
	}

	return &apiResponse, nil
}
