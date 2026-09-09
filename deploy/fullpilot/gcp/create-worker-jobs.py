#!/usr/bin/env python3
"""Create (or update) the warmbly-worker Cloud Run Job in each region via the
Cloud Run Admin API v2. Used instead of `gcloud run jobs create` because the
installed SDK predates Direct VPC egress flags for jobs.

  python3 create-worker-jobs.py            # create/update all regions
  python3 create-worker-jobs.py us-east1   # one region

Reads worker-env-<region>.yaml next to this file for plain env vars. Secrets
come from Secret Manager (see SECRETS below). Requires `gcloud auth login`.
"""
import json, os, subprocess, sys, time, urllib.request, urllib.error

PROJECT = "data-286013"
JOB = "warmbly-worker"
IMAGE = os.environ.get("WORKER_IMAGE", "us-docker.pkg.dev/data-286013/ghcr-remote/warmbly/warmbly/worker:v0.4.1")
SA = "warmbly-worker@data-286013.iam.gserviceaccount.com"
REGIONS = ["us-central1", "us-east1", "us-west1", "europe-west1"]
SECRETS = {
    "INTERNAL_API_TOKEN": "warmbly-internal-api-token",
    "ENCRYPTED_KEYS_WORKER_TOKEN": "warmbly-internal-api-token",
    "KMS_LOCAL_MASTER_KEY": "warmbly-kms-master-key",
    "CREDENTIALS_ENCRYPTION_KEY": "warmbly-credentials-key",
    "AWS_ACCESS_KEY_ID": "warmbly-blob-access-key",
    "AWS_SECRET_ACCESS_KEY": "warmbly-blob-secret-key",
    "BOX_GOOGLE_CLIENT_SECRET": "warmbly-box-google-client-secret",
}
# End the worker cleanly at 290s so the task counts as success; real crashes still fail.
CMD = ["/bin/sh"]
ARGS = ["-c", "timeout -s TERM 290 /app/worker; rc=$?; [ $rc -eq 124 ] && exit 0; exit $rc"]

HERE = os.path.dirname(os.path.abspath(__file__))
GCLOUD = os.environ.get("GCLOUD", "gcloud")


def token():
    return subprocess.check_output([GCLOUD, "auth", "print-access-token"], text=True).strip()


def env_from_yaml(path):
    out = []
    for line in open(path):
        line = line.strip()
        if not line or line.startswith("#") or ":" not in line:
            continue
        k, v = line.split(":", 1)
        out.append({"name": k.strip(), "value": v.strip().strip('"')})
    return out


def body(region):
    env = env_from_yaml(os.path.join(HERE, f"worker-env-{region}.yaml"))
    env += [{"name": k, "valueSource": {"secretKeyRef": {"secret": s, "version": "latest"}}} for k, s in SECRETS.items()]
    return {
        "template": {
            "taskCount": 2,
            "parallelism": 2,
            "template": {
                "serviceAccount": SA,
                "timeout": "330s",
                "maxRetries": 0,
                "containers": [{
                    "image": IMAGE,
                    "command": CMD,
                    "args": ARGS,
                    "env": env,
                    "resources": {"limits": {"cpu": "1", "memory": "512Mi"}},
                }],
                "vpcAccess": {
                    "networkInterfaces": [{"network": "default", "subnetwork": "default"}],
                    "egress": "PRIVATE_RANGES_ONLY",
                },
            },
        }
    }


def ssl_ctx():
    import ssl
    try:
        import certifi
        return ssl.create_default_context(cafile=certifi.where())
    except ImportError:
        for p in ("/etc/ssl/cert.pem", "/opt/homebrew/etc/ca-certificates/cert.pem"):
            if os.path.exists(p):
                return ssl.create_default_context(cafile=p)
    return ssl.create_default_context()


CTX = ssl_ctx()


def call(method, url, data=None, tok=None):
    req = urllib.request.Request(url, method=method, data=json.dumps(data).encode() if data else None)
    req.add_header("Authorization", f"Bearer {tok}")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, context=CTX) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        return {"error": e.code, "detail": e.read().decode()[:600]}


def main():
    regions = sys.argv[1:] or REGIONS
    tok = token()
    ops = {}
    for r in regions:
        base = f"https://run.googleapis.com/v2/projects/{PROJECT}/locations/{r}/jobs"
        exists = "error" not in call("GET", f"{base}/{JOB}", tok=tok)
        if exists:
            res = call("PATCH", f"{base}/{JOB}", body(r), tok)
            verb = "update"
        else:
            res = call("POST", f"{base}?jobId={JOB}", body(r), tok)
            verb = "create"
        if "error" in res:
            print(f"{r}: {verb} FAILED {res['error']}: {res['detail']}")
            continue
        ops[r] = res["name"]
        print(f"{r}: {verb} started")
    for r, op in ops.items():
        for _ in range(60):
            st = call("GET", f"https://run.googleapis.com/v2/{op}", tok=tok)
            if st.get("done"):
                err = st.get("error")
                print(f"{r}: {'ERROR ' + json.dumps(err)[:400] if err else 'ready'}")
                break
            time.sleep(3)
        else:
            print(f"{r}: still running after 3 min, check console")


if __name__ == "__main__":
    main()
