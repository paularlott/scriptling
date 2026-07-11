package nomad

import (
	"context"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.nomad"
	LibraryDesc = "HashiCorp Nomad client library covering CSI volumes and jobs"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// Register registers the scriptling.nomad library with the given registrar.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	b := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	b.FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, addr string) (object.Object, error) {
		token := kwargs.MustGetString("token", "")
		insecure := kwargs.MustGetBool("insecure", false)
		timeoutSecs := kwargs.MustGetFloat("timeout", DefaultTimeout.Seconds())
		c := newClient(addr, token, insecure, time.Duration(timeoutSecs*float64(time.Second)))
		return newClientInstance(c), nil
	}, `Client(addr, **kwargs) - Create a Nomad client

Parameters:
  addr (str): Nomad HTTP API address, e.g. "http://127.0.0.1:4646"
  token (str, optional): ACL token, sent as the X-Nomad-Token header. Default: ""
  insecure (bool, optional): Skip TLS certificate verification. Default: False
  timeout (float, optional): Per-request HTTP timeout in seconds. Default: 10

Returns:
  NomadClient: A client instance

Example:
  c = nomad.Client("https://nomad.example.com:4646", token="secret")
  c = nomad.Client("https://nomad.example.com:4646", token="secret", timeout=5)`)

	return b.Build()
}

// runBlockingErr runs fn with the interpreter lock released so shared-env
// threads can run while we wait on the Nomad API.
func runBlockingErr(ctx context.Context, fn func() error) error {
	var err error
	object.RunBlocking(ctx, func() { err = fn() })
	return err
}

func runBlockingVal[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var v T
	var err error
	object.RunBlocking(ctx, func() { v, err = fn() })
	return v, err
}

// ── NomadClient class ────────────────────────────────────────────────────────

type clientInstance struct {
	c *client
}

var (
	nomadClientClass     *object.Class
	nomadClientClassOnce sync.Once
)

func getNomadClientClass() *object.Class {
	nomadClientClassOnce.Do(func() {
		nomadClientClass = buildClientClass()
	})
	return nomadClientClass
}

func newClientInstance(c *client) *object.Instance {
	return object.NewInstanceWithFields(getNomadClientClass(), map[string]object.Object{
		"_client": &object.ClientWrapper{
			TypeName: "NomadClient",
			Client:   &clientInstance{c: c},
		},
	})
}

func getClientInstance(self *object.Instance) (*clientInstance, *object.Error) {
	wrapper, ok := object.GetClientField(self, "_client")
	if !ok || wrapper.Client == nil {
		return nil, &object.Error{Message: "NomadClient: missing internal client"}
	}
	ci, ok := wrapper.Client.(*clientInstance)
	if !ok {
		return nil, &object.Error{Message: "NomadClient: invalid internal client"}
	}
	return ci, nil
}

func buildClientClass() *object.Class {
	cb := object.NewClassBuilder("NomadClient")

	// ── CSI Volumes ──────────────────────────────────────────────────────────

	cb.MethodWithHelp("csi_volumes_list", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "*")
		pluginID := kwargs.MustGetString("plugin_id", "")
		vols, err := runBlockingVal(ctx, func() ([]CSIVolumeListEntry, error) {
			return ci.c.CSIVolumesList(ctx, namespace, pluginID)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		elements := make([]object.Object, len(vols))
		for i, v := range vols {
			elements[i] = object.NewStringDict(map[string]object.Object{
				"id":                  object.NewString(v.ID),
				"name":                object.NewString(v.Name),
				"namespace":           object.NewString(v.Namespace),
				"plugin_id":           object.NewString(v.PluginID),
				"provider":            object.NewString(v.Provider),
				"schedulable":         object.NewBoolean(v.Schedulable),
				"controllers_healthy": object.NewInteger(int64(v.ControllersHealthy)),
				"nodes_healthy":       object.NewInteger(int64(v.NodesHealthy)),
			})
		}
		return &object.List{Elements: elements}
	}, `csi_volumes_list(**kwargs) - List CSI volumes

Parameters:
  namespace (str, optional): Namespace to list, "*" for all namespaces. Default: "*"
  plugin_id (str, optional): Filter by CSI plugin ID. Default: "" (no filter)

Returns:
  list: List of dicts with {id, name, namespace, plugin_id, provider,
        schedulable, controllers_healthy, nodes_healthy}

Example:
  for v in c.csi_volumes_list(plugin_id="ceph-csi"):
    if v["id"].startswith("qaannon") or v["id"].startswith("qaprod"):
      print(v["id"])`)

	cb.MethodWithHelp("csi_volume_get", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		vol, err := runBlockingVal(ctx, func() (map[string]any, error) {
			return ci.c.CSIVolumeGet(ctx, id, namespace)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(vol)
	}, `csi_volume_get(id, **kwargs) - Get details for a CSI volume

Parameters:
  id (str): Volume ID
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)

Returns:
  dict: Full volume specification and status, as returned by the Nomad API

Example:
  vol = c.csi_volume_get("qaprod-data-01")
  print(vol["Provider"])`)

	cb.MethodWithHelp("csi_volume_register", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string, volume object.Object) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		volMap, ok := conversion.ToGo(volume).(map[string]any)
		if !ok {
			return &object.Error{Message: "csi_volume_register: volume must be a dict"}
		}
		if err := runBlockingErr(ctx, func() error { return ci.c.CSIVolumeRegister(ctx, id, namespace, volMap) }); err != nil {
			return &object.Error{Message: err.Error()}
		}
		return &object.Null{}
	}, `csi_volume_register(id, volume, **kwargs) - Register (create) a CSI volume

Parameters:
  id (str): Volume ID
  volume (dict): Volume specification in Nomad's CSI volume JSON format
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)

Returns:
  None

Example:
  c.csi_volume_register("qaprod-data-01", {
    "Name": "qaprod-data-01",
    "PluginID": "ceph-csi",
    "Capacity": 10 * 1024 * 1024 * 1024,
    "AccessMode": "single-node-writer",
    "AttachmentMode": "file-system",
  })`)

	cb.MethodWithHelp("csi_volume_deregister", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		force := kwargs.MustGetBool("force", false)
		if err := runBlockingErr(ctx, func() error { return ci.c.CSIVolumeDeregister(ctx, id, namespace, force) }); err != nil {
			return &object.Error{Message: err.Error()}
		}
		return &object.Null{}
	}, `csi_volume_deregister(id, **kwargs) - Deregister (delete) a CSI volume

Parameters:
  id (str): Volume ID
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)
  force (bool, optional): Force detach any remaining claims first. Default: False

Returns:
  None

Example:
  c.csi_volume_deregister("qaprod-orphaned-01", force=True)`)

	// ── Jobs ─────────────────────────────────────────────────────────────────

	cb.MethodWithHelp("jobs_list", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "*")
		prefix := kwargs.MustGetString("prefix", "")
		jobs, err := runBlockingVal(ctx, func() ([]JobListEntry, error) {
			return ci.c.JobsList(ctx, namespace, prefix)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		elements := make([]object.Object, len(jobs))
		for i, j := range jobs {
			elements[i] = object.NewStringDict(map[string]object.Object{
				"id":        object.NewString(j.ID),
				"name":      object.NewString(j.Name),
				"namespace": object.NewString(j.Namespace),
				"type":      object.NewString(j.Type),
				"status":    object.NewString(j.Status),
				"priority":  object.NewInteger(int64(j.Priority)),
			})
		}
		return &object.List{Elements: elements}
	}, `jobs_list(**kwargs) - List jobs

Parameters:
  namespace (str, optional): Namespace to list, "*" for all namespaces. Default: "*"
  prefix (str, optional): Filter by job ID prefix. Default: "" (no filter)

Returns:
  list: List of dicts with {id, name, namespace, type, status, priority}

Example:
  for j in c.jobs_list(prefix="qaannon"):
    print(j["id"], j["status"])`)

	cb.MethodWithHelp("job_get", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		job, err := runBlockingVal(ctx, func() (map[string]any, error) {
			return ci.c.JobGet(ctx, id, namespace)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(job)
	}, `job_get(id, **kwargs) - Get the full specification and status for a job

Parameters:
  id (str): Job ID
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)

Returns:
  dict: Job specification and status, as returned by the Nomad API

Example:
  job = c.job_get("qaprod-api")
  print(job["Status"])`)

	cb.MethodWithHelp("job_register", func(self *object.Instance, ctx context.Context, job object.Object) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		jobMap, ok := conversion.ToGo(job).(map[string]any)
		if !ok {
			return &object.Error{Message: "job_register: job must be a dict"}
		}
		result, err := runBlockingVal(ctx, func() (map[string]any, error) { return ci.c.JobRegister(ctx, jobMap) })
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(result)
	}, `job_register(job) - Register (create or update) a job

Parameters:
  job (dict): Job specification in Nomad's JSON job format (e.g. from job_parse()
              or job_get()["Job"])

Returns:
  dict: Registration response with {EvalID, EvalCreateIndex, JobModifyIndex, Warnings}

Example:
  parsed = c.jobs_parse(hcl_text)
  result = c.job_register(parsed)
  print(result["EvalID"])`)

	cb.MethodWithHelp("job_stop", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		purge := kwargs.MustGetBool("purge", false)
		result, err := runBlockingVal(ctx, func() (map[string]any, error) {
			return ci.c.JobStop(ctx, id, namespace, purge)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(result)
	}, `job_stop(id, **kwargs) - Stop a job

Parameters:
  id (str): Job ID
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)
  purge (bool, optional): Fully remove the job from Nomad's state instead of
                          leaving it stopped. Default: False

Returns:
  dict: Stop response with {EvalID, EvalCreateIndex, JobModifyIndex}

Example:
  c.job_stop("qaprod-old-job", purge=True)`)

	cb.MethodWithHelp("wait_job_stopped", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		namespace := kwargs.MustGetString("namespace", "")
		timeoutSecs := kwargs.MustGetInt("timeout", 30)
		stopped, err := runBlockingVal(ctx, func() (bool, error) {
			return ci.c.WaitJobStopped(ctx, id, namespace, time.Duration(timeoutSecs)*time.Second)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return object.NewBoolean(stopped)
	}, `wait_job_stopped(id, **kwargs) - Wait for a job to reach the "dead" status

Parameters:
  id (str): Job ID
  namespace (str, optional): Namespace. Default: "" (Nomad default namespace)
  timeout (int, optional): Maximum time to wait in seconds. Default: 30

Returns:
  bool: True if the job is stopped, False if the timeout was reached

Example:
  c.job_stop("qaprod-old-job")
  if not c.wait_job_stopped("qaprod-old-job", timeout=60):
    print("job did not stop in time")`)

	cb.MethodWithHelp("job_validate", func(self *object.Instance, ctx context.Context, job object.Object) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		jobMap, ok := conversion.ToGo(job).(map[string]any)
		if !ok {
			return &object.Error{Message: "job_validate: job must be a dict"}
		}
		result, err := runBlockingVal(ctx, func() (map[string]any, error) { return ci.c.JobValidate(ctx, jobMap) })
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(result)
	}, `job_validate(job) - Validate a job specification without submitting it

Parameters:
  job (dict): Job specification in Nomad's JSON job format

Returns:
  dict: Validation result with {DriverConfigValidated, ValidationErrors, Warnings}

Example:
  result = c.job_validate(parsed_job)
  if result["ValidationErrors"]:
    print(result["ValidationErrors"])`)

	cb.MethodWithHelp("job_plan", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, id string, job object.Object) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		diff := kwargs.MustGetBool("diff", false)
		jobMap, ok := conversion.ToGo(job).(map[string]any)
		if !ok {
			return &object.Error{Message: "job_plan: job must be a dict"}
		}
		result, err := runBlockingVal(ctx, func() (map[string]any, error) { return ci.c.JobPlan(ctx, id, jobMap, diff) })
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(result)
	}, `job_plan(id, job, **kwargs) - Dry-run a job registration and return the scheduler plan

Parameters:
  id (str): Job ID
  job (dict): Job specification in Nomad's JSON job format
  diff (bool, optional): Include a diff against the current job version. Default: False

Returns:
  dict: Plan result with {JobModifyIndex, Annotations, FailedTGAllocs, ...}

Example:
  plan = c.job_plan("qaprod-api", parsed_job, diff=True)`)

	cb.MethodWithHelp("jobs_parse", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, hcl string) object.Object {
		ci, errObj := getClientInstance(self)
		if errObj != nil {
			return errObj
		}
		canonicalize := kwargs.MustGetBool("canonicalize", false)
		result, err := runBlockingVal(ctx, func() (map[string]any, error) {
			return ci.c.JobsParse(ctx, hcl, canonicalize)
		})
		if err != nil {
			return &object.Error{Message: err.Error()}
		}
		return conversion.FromGo(result)
	}, `jobs_parse(hcl, **kwargs) - Convert an HCL job specification into Nomad's JSON job format

Parameters:
  hcl (str): Job specification in HCL format
  canonicalize (bool, optional): Fill in default values for optional fields. Default: False

Returns:
  dict: Job specification in Nomad's JSON job format, suitable for job_register(),
        job_validate(), or job_plan()

Example:
  parsed = c.jobs_parse(open("job.nomad.hcl").read())
  c.job_register(parsed)`)

	return cb.Build()
}
