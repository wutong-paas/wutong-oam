package export

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wutong-paas/wutong-oam/pkg/ram/v1alpha1"
	"github.com/wutong-paas/wutong-oam/pkg/util/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type helmChartExporter struct {
	logger      *logrus.Logger
	ram         v1alpha1.WutongApplicationConfig
	imageClient image.Client
	mode        string
	homePath    string
	exportPath  string
}

func (h *helmChartExporter) Export() (*Result, error) {
	h.logger.Infof("start export app %s to helm chart spec", h.ram.AppName)
	dependentImages, err := h.initHelmChart()
	if err != nil {
		return nil, err
	}
	if err := SaveComponents(h.ram, h.imageClient, h.exportPath, h.logger, dependentImages); err != nil {
		h.logger.Errorf("helm chart export save component failure %v", err)
		return nil, err
	}
	h.logger.Infof("success save components")
	// Save plugin attachments
	if err := SavePlugins(h.ram, h.imageClient, h.exportPath, h.logger); err != nil {
		return nil, err
	}
	h.logger.Infof("success save plugins")

	packageName := fmt.Sprintf("%s-%s-helm.tar.gz", h.ram.AppName, h.ram.AppVersion)
	name, err := Packaging(packageName, h.homePath, h.exportPath)
	if err != nil {
		err = fmt.Errorf("failed to package app %s: %s", packageName, err.Error())
		h.logger.Error(err)
		return nil, err
	}
	h.logger.Infof("success export app " + h.ram.AppName)
	return &Result{PackagePath: path.Join(h.homePath, name), PackageName: name}, nil
}

func (h *helmChartExporter) initHelmChart() ([]string, error) {
	helmChartPath := path.Join(h.exportPath, h.ram.AppName)
	err := h.writeChartYaml(helmChartPath)
	if err != nil {
		h.logger.Errorf("%v writeChartYaml failure %v", h.ram.AppName, err)
		return nil, err
	}
	h.logger.Infof("writeChartYaml success")
	for i := 0; i < 40; i++ {
		time.Sleep(1 * time.Second)
		if CheckFileExist(path.Join(helmChartPath, "dependent_image.txt")) {
			h.logger.Infof("dependent_image.txt creeate success")
			break
		}
	}

	content, err := os.ReadFile(path.Join(helmChartPath, "dependent_image.txt"))
	if err != nil {
		h.logger.Errorf("read file dependent_image.txt failure %v", err)
		return nil, err
	}
	dependentImages := strings.Split(string(content), "\n")
	err = h.writeTemplateYaml(helmChartPath)
	if err != nil {
		return nil, err
	}
	return dependentImages, nil

}

type ChartYaml struct {
	ApiVersion  string `json:"apiVersion,omitempty"`
	AppVersion  string `json:"appVersion,omitempty"`
	Description string `json:"description,omitempty"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Version     string `json:"version,omitempty"`
}

func (h *helmChartExporter) writeChartYaml(helmChartPath string) error {
	cy := ChartYaml{
		ApiVersion:  "v2",
		AppVersion:  h.ram.AppVersion,
		Description: h.ram.Annotations["version_info"],
		Name:        h.ram.AppName,
		Type:        "application",
		Version:     h.ram.AppVersion,
	}
	cyYaml, err := yaml.Marshal(cy)
	if err != nil {
		return err
	}
	return h.write(path.Join(helmChartPath, "Chart.yaml"), cyYaml)
}

func (h *helmChartExporter) writeTemplateYaml(helmChartPath string) error {
	helmChartTemplatePath := path.Join(helmChartPath, "templates")
	for _, k8sResource := range h.ram.K8sResources {
		var unstructuredObject unstructured.Unstructured
		err := yaml.Unmarshal([]byte(k8sResource.Content), &unstructuredObject)
		if err != nil {
			return err
		}
		unstructuredObject.SetNamespace("")
		unstructuredObject.SetResourceVersion("")
		unstructuredObject.SetCreationTimestamp(metav1.Time{})
		unstructuredObject.SetUID("")
		unstructuredYaml, err := yaml.Marshal(&unstructuredObject)
		if err != nil {
			return err
		}
		err = h.write(path.Join(helmChartTemplatePath, fmt.Sprintf("%v.yaml", unstructuredObject.GetKind())), unstructuredYaml)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *helmChartExporter) write(helmChartFilePath string, meta []byte) error {
	var fl *os.File
	var err error
	if CheckFileExist(helmChartFilePath) {
		fl, err = os.OpenFile(helmChartFilePath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
	} else {
		fl, err = os.Create(helmChartFilePath)
		if err != nil {
			return err
		}
	}
	defer fl.Close()
	n, err := fl.Write(append(meta, []byte("\n---\n")...))
	if err != nil {
		return err
	}
	if n < len(append(meta, []byte("\n---")...)) {
		return fmt.Errorf("write insufficient length")
	}
	return nil
}
