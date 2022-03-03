// WUTONG, Application Management Platform
// Copyright (C) 2020-2020 Wutong Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Wutong,
// one or multiple Commercial Licenses authorized by Wutong Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package export

import (
	"fmt"
	"path"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/wutong-paas/wutong-oam/pkg/ram/v1alpha1"
)

//AppLocalExport export local package
type AppLocalExport interface {
	Export() (*Result, error)
}

//Result export result
type Result struct {
	PackagePath   string
	PackageName   string
	PackageFormat string
}

//AppFormat app spec format
type AppFormat string

var (
	//RAM -
	RAM AppFormat = "ram"
	//DC -
	DC AppFormat = "docker-compose"
)

//New new exporter
func New(format AppFormat, homePath string, ram v1alpha1.WutongApplicationConfig, client *client.Client, logger *logrus.Logger) AppLocalExport {
	switch format {
	case RAM:
		return &ramExporter{
			logger:     logger,
			ram:        ram,
			client:     client,
			mode:       "offline",
			homePath:   homePath,
			exportPath: path.Join(homePath, fmt.Sprintf("%s-%s-ram", ram.AppName, ram.AppVersion)),
		}
	case DC:
		return &dockerComposeExporter{
			logger:     logger,
			ram:        ram,
			client:     client,
			homePath:   homePath,
			exportPath: path.Join(homePath, fmt.Sprintf("%s-%s-dockercompose", ram.AppName, ram.AppVersion)),
		}
	default:
		panic("not support app format")
	}
}
