package handler

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/xsnout/ursa/cmd/utils"
	"github.com/xsnout/ursa/pkg/model"
)

const (
	WebSocketURLPrefix            = "ws://"
	WebSocketClientBinary         = "demo-pipe-client"
	WebSocketServerBinary         = "demo-pipe-server"
	DashboardBinary               = "demo-web-server"
	DemoFinancialDataServerBinary = "demo-findata-server"
	DemoSyslogBinary              = "syslog"
	DemoThrottleBinary            = "throttle"
	DemoGeneratorBinary           = "generator"
	DashboardTemplateFile         = "dashboard-template.html"
)

var (
	log                    zerolog.Logger
	templatesDirectoryPath string
	demoDirectoryPath      string
)

func init() {
	//zerolog.SetGlobalLevel(zerolog.Disabled)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log = zerolog.New(os.Stderr).With().Caller().Timestamp().Logger()
	log.Info().Msg("Handler says welcome!")

	demoDirectoryPath = filepath.Join("/", "tmp", "demo")
	//templatesDirectoryPath = filepath.Join("/", "tmp", "repos", "ursa", "templates")
	templatesDirectoryPath = filepath.Join("/", "tmp", "demo", "templates")

}

type JobHandler interface {
	Add(*gin.Context)
	Delete(*gin.Context)
	List(*gin.Context)
	Start(*gin.Context)
	Stop(*gin.Context)
}

type EngineCompartment struct {
	JobId string
}

type jobHandler struct {
	cfg utils.Configuration
}

func NewJobHandler(cfg utils.Configuration) JobHandler {
	return &jobHandler{
		cfg: cfg,
	}
}

func (app *jobHandler) List(c *gin.Context) {
	_, ctxErr := context.WithTimeout(c.Request.Context(), time.Duration(app.cfg.App.Timeout)*time.Second)
	defer ctxErr()

	jobs := make([]model.Job, 0, len(app.cfg.App.Jobs))
	for _, v := range app.cfg.App.Jobs {
		jobs = append(jobs, v)
	}

	res := model.ListJobsResponse{
		Jobs: jobs,
	}
	c.JSON(http.StatusOK, res)
}

func (app *jobHandler) Start(c *gin.Context) {
	_, ctxErr := context.WithTimeout(c.Request.Context(), time.Duration(app.cfg.App.Timeout)*time.Second)
	defer ctxErr()

	req := model.StartRequest{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	fmt.Printf("START 1: %v\n", req)

	var job model.Job
	var ok bool
	if job, ok = app.cfg.App.Jobs[req.Id]; !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("cannot find job with ID %s", req.Id))
		return
	}

	url := "http://localhost:" + strconv.Itoa(job.DashboardPort)
	fmt.Printf("Dashboard URL: %s\n", url)

	var statusMsg string
	var err error
	if statusMsg, err = startEngine(job); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	res := model.StartResponse{
		DashboardURL: url,
		Message:      statusMsg,
	}
	c.JSON(http.StatusOK, res)
}

func (app *jobHandler) Stop(c *gin.Context) {
	_, ctxErr := context.WithTimeout(c.Request.Context(), time.Duration(app.cfg.App.Timeout)*time.Second)
	defer ctxErr()

	req := model.StopRequest{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var job model.Job
	var ok bool
	if job, ok = app.cfg.App.Jobs[req.Id]; !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("cannot find job with ID %s", req.Id))
		return
	}

	var statusMsg string
	var err error
	if statusMsg, err = stopEngine(job.JobDirectoryPath); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	res := model.StopResponse{
		Id:      job.Id,
		Message: statusMsg,
	}
	c.JSON(http.StatusOK, res)
}

func (app *jobHandler) Delete(c *gin.Context) {
	_, ctxErr := context.WithTimeout(c.Request.Context(), time.Duration(app.cfg.App.Timeout)*time.Second)
	defer ctxErr()

	req := model.DeleteRequest{}
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var job model.Job
	var ok bool
	if job, ok = app.cfg.App.Jobs[req.Id]; !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("cannot find job with ID %s", req.Id))
	}

	// Return the reserved ports back to the pool.
	app.cfg.App.AvailablePorts = append(app.cfg.App.AvailablePorts, job.Pipe1IngressPort)
	app.cfg.App.AvailablePorts = append(app.cfg.App.AvailablePorts, job.Pipe1EgressPort)
	app.cfg.App.AvailablePorts = append(app.cfg.App.AvailablePorts, job.Pipe2IngressPort)
	app.cfg.App.AvailablePorts = append(app.cfg.App.AvailablePorts, job.Pipe2EgressPort)
	app.cfg.App.AvailablePorts = append(app.cfg.App.AvailablePorts, job.DashboardPort)

	delete(app.cfg.App.Jobs, req.Id)
	c.JSON(http.StatusOK, "success")
}

func (app *jobHandler) Add(c *gin.Context) {
	_, ctxErr := context.WithTimeout(c.Request.Context(), time.Duration(app.cfg.App.Timeout)*time.Second)
	defer ctxErr()

	jobRequest := model.AddJobRequest{}
	if err := c.ShouldBindBodyWith(&jobRequest, binding.JSON); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id := uuid.NewString()
	path := createJobDirectory(id)

	if len(app.cfg.App.AvailablePorts) < 5 {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("not enough ports available"))
		return
	}

	pipe1IngressPort := app.cfg.App.AvailablePorts[0]
	app.cfg.App.AvailablePorts = app.cfg.App.AvailablePorts[1:]
	pipe1EgressPort := app.cfg.App.AvailablePorts[0]
	app.cfg.App.AvailablePorts = app.cfg.App.AvailablePorts[1:]
	pipe2IngressPort := app.cfg.App.AvailablePorts[0]
	app.cfg.App.AvailablePorts = app.cfg.App.AvailablePorts[1:]
	pipe2EgressPort := app.cfg.App.AvailablePorts[0]
	app.cfg.App.AvailablePorts = app.cfg.App.AvailablePorts[1:]
	dashboardPort := app.cfg.App.AvailablePorts[0]
	app.cfg.App.AvailablePorts = app.cfg.App.AvailablePorts[1:]

	job := model.Job{
		Id:                    id,
		QueryId:               jobRequest.QueryId,
		CatalogId:             jobRequest.CatalogId,
		SpoutId:               jobRequest.SpoutId,
		PrepId:                jobRequest.PrepId,
		Created:               fmt.Sprint(time.Now().UTC().Format(time.RFC3339Nano)),
		Pipe1IngressPort:      pipe1IngressPort,
		Pipe1EgressPort:       pipe1EgressPort,
		Pipe2IngressPort:      pipe2IngressPort,
		Pipe2EgressPort:       pipe2EgressPort,
		EnginePath:            filepath.Join("/", "tmp", "jobs", id, "grizzly"),
		BinaryPlanPath:        filepath.Join("/", "tmp", "jobs", id, "plan.bin"),
		LogFilePath:           filepath.Join("grizzly.log"),
		SampleCSVFilePath:     filepath.Join("sample.csv"),
		SpoutPath:             filepath.Join("/", "tmp", "jobs", id, "spout.cmd"),
		DemoFinDataServerPath: filepath.Join("/", "tmp", "jobs", id, DemoFinancialDataServerBinary),
		DemoSyslogPath:        filepath.Join("/", "tmp", "jobs", id, DemoSyslogBinary),
		DemoThrottlePath:      filepath.Join("/", "tmp", "jobs", id, DemoThrottleBinary),
		DashboardPort:         dashboardPort,
		DashboardURL:          "http://localhost:" + strconv.Itoa(dashboardPort),
		ExitAfterSeconds:      3600,
		ReaderWebSocket:       WebSocketURLPrefix + ":",
		WriterWebSocket:       WebSocketURLPrefix + ":",
		JobDirectoryPath:      path,
	}
	app.cfg.App.Jobs[job.Id] = job

	var err error
	if err = copyFile(CatalogsDirectoryPath+"/"+job.CatalogId+".json", job.JobDirectoryPath+"/catalog.json", 0644); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(QueriesDirectoryPath+"/"+job.QueryId+".uql", job.JobDirectoryPath+"/query.uql", 0644); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(SpoutsDirectoryPath+"/"+job.SpoutId+".cmd", job.JobDirectoryPath+"/spout.cmd", 0644); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(PrepsDirectoryPath+"/"+job.PrepId+".sh", job.JobDirectoryPath+"/prep.sh", 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	if err = copyFile(demoDirectoryPath+"/"+WebSocketClientBinary, job.JobDirectoryPath+"/"+WebSocketClientBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+WebSocketServerBinary, job.JobDirectoryPath+"/"+WebSocketServerBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+DashboardBinary, job.JobDirectoryPath+"/"+DashboardBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+DemoFinancialDataServerBinary, job.JobDirectoryPath+"/"+DemoFinancialDataServerBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+DemoSyslogBinary, job.JobDirectoryPath+"/"+DemoSyslogBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+DemoThrottleBinary, job.JobDirectoryPath+"/"+DemoThrottleBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = copyFile(demoDirectoryPath+"/"+DemoGeneratorBinary, job.JobDirectoryPath+"/"+DemoGeneratorBinary, 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	if err = copyFile(templatesDirectoryPath+"/"+DashboardTemplateFile, job.JobDirectoryPath+"/"+DashboardTemplateFile, 0644); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	// if err = copyFile(templatesDirectoryPath+"/"+"run-console-template.sh", job.JobDirectoryPath+"/"+"run-console-template.sh", 0644); err != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, err)
	// }
	// if err = copyFile(templatesDirectoryPath+"/"+"run-dashboard-template.sh", job.JobDirectoryPath+"/"+"run-dashboard-template.sh", 0644); err != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, err)
	// }

	var statusMsg string
	if statusMsg, err = prepare(job.JobDirectoryPath); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	} else if statusMsg, err = buildEngine(job.JobDirectoryPath); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	app.cfg.App.JobChan <- job

	if err = createSampleRunScript(job, templatesDirectoryPath+"/run-console-template.sh", job.JobDirectoryPath+"/run-console.sh", 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	if err = createSampleRunScript(job, templatesDirectoryPath+"/run-dashboard-template.sh", job.JobDirectoryPath+"/run-dashboard.sh", 0755); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}
	// if err = createSampleRunScript(job, job.JobDirectoryPath+"/run-console-template.sh", job.JobDirectoryPath+"/run-console.sh", 0755); err != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, err)
	// }
	// if err = createSampleRunScript(job, job.JobDirectoryPath+"/run-dashboard-template.sh", job.JobDirectoryPath+"/run-dashboard.sh", 0755); err != nil {
	// 	c.AbortWithError(http.StatusInternalServerError, err)
	// }

	res := model.AddResponse{
		Id:      job.Id,
		Message: statusMsg,
	}
	c.JSON(http.StatusOK, res)
}

// Replace some placeholders in the template file with actual values.
func createSampleRunScript(job model.Job, src string, dst string, perm fs.FileMode) (err error) {
	dataSpout := readFile(job.SpoutPath)

	s := readFile(src)
	s = strings.Replace(s, "@@@PIPE_1_INGRESS_PORT@@@", strconv.Itoa(job.Pipe1IngressPort), -1)
	s = strings.Replace(s, "@@@PIPE_1_EGRESS_PORT@@@", strconv.Itoa(job.Pipe1EgressPort), -1)
	s = strings.Replace(s, "@@@PIPE_2_INGRESS_PORT@@@", strconv.Itoa(job.Pipe2IngressPort), -1)
	s = strings.Replace(s, "@@@PIPE_2_EGRESS_PORT@@@", strconv.Itoa(job.Pipe2EgressPort), -1)
	s = strings.Replace(s, "@@@DASHBOARD_PORT@@@", strconv.Itoa(job.DashboardPort), -1)
	//s = strings.Replace(s, "@@@CSV_FILE@@@", job.SampleCSVFilePath, -1)
	s = strings.Replace(s, "@@@JOB_LOG@@@", job.LogFilePath, -1)
	//s = strings.Replace(s, "@@@THROTTLE_MILLISECONDS@@@", strconv.Itoa(job.ThrottleMilliseconds), -1)
	s = strings.Replace(s, "@@@EXIT_AFTER_SECONDS@@@", strconv.Itoa(job.ExitAfterSeconds), -1)
	s = strings.Replace(s, "@@@DATA_SPOUT@@@", dataSpout, -1)

	if err = os.WriteFile(dst, []byte(s), perm); err != nil {
		return
	}

	if err = os.Remove(src); err != nil {
		return
	}

	return
}

func readFile(path string) string {
	var bytes []byte
	var err error
	if bytes, err = os.ReadFile(path); err != nil {
		panic(err)
	}
	return string(bytes)
}

func copyFile(src string, dst string, perm fs.FileMode) (err error) {
	var bytesRead []byte
	if bytesRead, err = os.ReadFile(src); err != nil {
		return
	}
	if err = os.WriteFile(dst, bytesRead, perm); err != nil {
		return
	}
	return
}

func createJobDirectory(id string) (path string) {
	path = filepath.Join("/", "tmp", "jobs", id)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		panic(err)
	}
	return
}

func prepare(jobDirectoryPath string) (statusMsg string, err error) {
	cmd := exec.Command("./prep.sh")
	cmd.Dir = jobDirectoryPath
	var out bytes.Buffer
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		statusMsg = "problem starting preparation script: " + err.Error()
		return
	}
	statusMsg = out.String()
	return
}

func buildEngine(jobDirectoryPath string) (statusMsg string, err error) {
	params := "JOB_DIR=" + jobDirectoryPath
	cmd := exec.Command("make", "all", params)
	cmd.Dir = "/tmp/repos/grizzly"

	var out bytes.Buffer
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		statusMsg = "problem building engine"
		return
	}
	statusMsg = out.String()
	return
}

func startEngine(job model.Job) (statusMsg string, err error) {
	cmd := exec.Command("./run-dashboard.sh")
	cmd.Dir = job.JobDirectoryPath
	var out bytes.Buffer
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		statusMsg = "problem starting engine: " + err.Error()
		return
	}
	statusMsg = out.String()
	return
}

func stopEngine(jobDirectoryPath string) (statusMsg string, err error) {
	var out bytes.Buffer

	cmd := exec.Command("pkill", "demo")
	cmd.Dir = jobDirectoryPath
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		statusMsg = "problem stopping demo"
		return
	}
	statusMsg += out.String()

	cmd = exec.Command("pkill", "grizzly")
	cmd.Dir = jobDirectoryPath
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		statusMsg = "problem stopping grizzly"
		return
	}
	statusMsg += out.String()

	return
}
