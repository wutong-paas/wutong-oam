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

package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/wutong-paas/wutong-oam/pkg/util/zip"
)

//NewUUID new uuid string
func NewUUID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

var reg = regexp.MustCompile(`(?U)\$\{.*\}`)

//ParseVariable parse and replace variable in source str
func ParseVariable(source string, configs map[string]string) string {
	resultKey := reg.FindAllString(source, -1)
	for _, sourcekey := range resultKey {
		key, defaultValue := getVariableKey(sourcekey)
		if value, ok := configs[key]; ok {
			source = strings.Replace(source, sourcekey, value, -1)
		} else if defaultValue != "" {
			source = strings.Replace(source, sourcekey, defaultValue, -1)
		}
	}
	return source
}

func getVariableKey(source string) (key, value string) {
	if len(source) < 4 {
		return "", ""
	}
	left := strings.Index(source, "{")
	right := strings.Index(source, "}")
	k := source[left+1 : right]
	if strings.Contains(k, ":") {
		re := strings.Split(k, ":")
		if len(re) > 1 {
			return re[0], re[1]
		}
		return re[0], ""
	}
	return k, ""
}

//Unzip archive file to target dir
func Unzip(archive, target string) error {
	reader, err := zip.OpenDirectReader(archive)
	if err != nil {
		return fmt.Errorf("error opening archive: %v", err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for _, file := range reader.File {
		run := func() error {
			path := filepath.Join(target, file.Name)
			if file.FileInfo().IsDir() {
				os.MkdirAll(path, file.Mode())
				if file.Comment != "" && strings.Contains(file.Comment, "/") {
					guid := strings.Split(file.Comment, "/")
					if len(guid) == 2 {
						uid, _ := strconv.Atoi(guid[0])
						gid, _ := strconv.Atoi(guid[1])
						if err := os.Chown(path, uid, gid); err != nil {
							return fmt.Errorf("error changing owner: %v", err)
						}
					}
				}
				return nil
			}

			fileReader, err := file.Open()
			if err != nil {
				return fmt.Errorf("fileReader; error opening file: %v", err)
			}
			defer fileReader.Close()
			targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return fmt.Errorf("targetFile; error opening file: %v", err)
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, fileReader); err != nil {
				return fmt.Errorf("error copy file: %v", err)
			}
			if file.Comment != "" && strings.Contains(file.Comment, "/") {
				guid := strings.Split(file.Comment, "/")
				if len(guid) == 2 {
					uid, _ := strconv.Atoi(guid[0])
					gid, _ := strconv.Atoi(guid[1])
					if err := os.Chown(path, uid, gid); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := run(); err != nil {
			return err
		}
	}

	return nil
}

//Untar tar -zxvf
func Untar(archive, target string) error {
	cmd := exec.Command("tar", "-xzf", archive, "-C", target)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

//GetFileList -
func GetFileList(dirpath string, level int) ([]string, error) {
	var dirlist []string
	list, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		if !f.IsDir() && level <= 1 {
			dirlist = append(dirlist, filepath.Join(dirpath, f.Name()))
		} else if level > 1 && f.IsDir() {
			list, err := GetFileList(filepath.Join(dirpath, f.Name()), level-1)
			if err != nil {
				return nil, err
			}
			dirlist = append(dirlist, list...)
		}
	}
	return dirlist, nil
}
