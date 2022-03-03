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

package docker

import (
	"os"
	"testing"

	"github.com/wutong-paas/wutong-oam/pkg/ram/v1alpha1"
)

func TestNewImage(t *testing.T) {
	new, err := NewImageName("hub.wutong-paas.com/655c233e59714d9191c0b9e856d84b44/405158ca2136824ffd7ad1df21529926:v2.0", v1alpha1.ImageInfo{
		HubPassword: os.Getenv("PASS"),
		Namespace:   "test",
		HubURL:      "image.wutong-paas.com",
		HubUser:     "root",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(new)
}

func TestGetOldSaveImageName(t *testing.T) {
	t.Log(GetOldSaveImageName("wutong-paas.me/23ehgni5/67b72db7c89259e14df990e5ca7941a3:20190805191011", false))
}
