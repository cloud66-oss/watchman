<img src="http://cdn2-cloud66-com.s3.amazonaws.com/images/oss-sponsorship.png" width=150/>

# Watchman
Watchman is a web service to check health of an HTTP(S) endpoint remotely. It is intendend to run as a light, multi-instnace service on different geographical locations to provide a full picture of a web endpoint's health and accessibilty. 

Watchman can easily be deployed as a "serverless" function (like AWS Lambda or GCP Cloud Run) for full geographical distribution, access to high quality network and cost effective monitoring.

## Usage
Watchman is a single exectuable and can run on Linux, Mac, Windows or any other platform where you can compile it for (it's written in Go, so it's pretty portable). Once running, it will accept check requests and return the check results. 

To use Watchman, first run it on your local machine: 

```bash
watchman
```

This will start the process and listen to port 8080. Now you can use curl to send check requests to it:

```bash
curl -H "Content-Type: application/json" http://localhost:8080 --data '{"url":"http://www.google.com","timeout":"1s"}'
```

```json 
{
  "status": 200,
  "error": "",
  "dns_lookup": 18228429,
  "tcp_connection": 10254574,
  "tls_handshake": 0,
  "server_processing": 67631930,
  "content_transfer": 561552,
  "name_lookup": 18228429,
  "connect": 28456431,
  "pre_transfer": 0,
  "start_transfer": 96114933,
  "total": 96676485
}
```

The command above, runs a check, from your local machine, to `http://www.google.com` with a 1 second timeout. If `http://www.google.com` responds in less than 1 second, Watchman will return a JSON payload with the site's response metrics. The numbers are in milliseconds.

If there is an error during the check (the site name is invalid or the site is down), the `status` will be `0` and the `error` will have more information on the error. For a successful check, you should always look for `200` in `status`.

`tls_handshake` and `pre_transfer` will only be populated for `https` requests.

### Request
Checks can be requested with the following `POST` payload to Watchman:

```json
{
    "url": "https://www.google.com",
    "timeout": "100ms",
    "redirects_to_follow": 3,
    "verify_certs": true
}
```

Only `url` is required. The rest of the attributes are defaulted to the values above.

### Response
Watchman returns the following payload:

```json
{
    "status": 200,
    "error": "",
    "dns_lookup": 18228429,
    "tcp_connection": 10254574,
    "tls_handshake": 0,
    "server_processing": 67631930,
    "content_transfer": 561552,
    "name_lookup": 18228429,
    "connect": 28456431,
    "pre_transfer": 0,
    "start_transfer": 96114933,
    "total": 96676485
}
```

When a check fails due to the site issues, `status` will be `0` and `error` will have more info on the error. The rest of the values will be `0`. 

A successful check will have `200` as `status` and an empty `error`. 

An internal error in Watchman will return a `500` status code and the text of the error.

## Configuration
Watchman is configured using environment variables. This makes it suitable for serverless environments (see below). The following environment variables are supported:

- `PORT`  Port to listen to (default 8080)
- `TIMEOUT` Default check timeout. (default 100ms)
- `MAX_REDIRECTS` Maximum number of HTTP redirects Watchman should follow before giving up (default 3)
- `AUTH_TOKEN`  If provided, Watchman will only respond to requests that have the same value in their X-Token header. (default none)
- `SENTRY_API` If provided, Watchman will send crash reports to Sentry.
- `_DEPLOY_REGION` Is used as an http header (`X-Region`) when calling the checked endpoint.

## Deployment
Watchman comes with a Dockerfile, so you can run it as a Docker container (ie on Kubernetes) or as a serverless deployment. Simply provide the required environment variables and start the executable. In production environment, make sure you secure the setup with SSL.

You can use the prebuilt image of Watchman:

```bash
docker pull cloud66/watchman
```


### Deploying to GCP Cloud Run
This is a quick guide on how to deploy Watchman to multiple Google Cloud Run regions.

> Make sure your `gcloud` is configured and connected to your GCP account correctly.

```bash
gcloud builds submit --tag gcr.io/PROJECT-ID/watchman
```

This will start the build process for the first time. Now you can deploy Watchman:


```bash
gcloud run deploy --image gcr.io/PROJECT-ID/watchman --platform managed
```

1. You will be prompted for the service name: press Enter to accept the default name, watchman.
2. You will be prompted for region: select the region of your choice, for example us-central1.
3. You will be prompted to allow unauthenticated invocations: respond y

Repeat this process with as many regions as you'd like. Make sure to set the environment variables under the **Variables** section for the services in GCP.

## Scheduling and Alerts
Running checks regularly and sending alerts when the endpoint is not reachable is not within the remit of Watchman, but it's easy to setup. You can use any hosted cloud scheduling services (like GCP Cloud Schedule) or your own cron jobs to do this by hitting Watchman endpoints and aggregating the restuls.