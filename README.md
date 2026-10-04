# Image Guard

Image Guard is a small Go alpha service that accepts images, queues moderation in SQLite, and checks each image with ShieldGemma through a llama.cpp server.

## Run

Use Go 1.25 or newer. Start a llama.cpp server separately with a ShieldGemma 2 compatible model and vision projector, then set its chat completions URL in `config.yaml` or with `IMAGE_GUARD_MODERATION_ENDPOINT`.

```sh
go run ./cmd/server
```

Open `http://localhost:8080/` to use the moderation workspace.

The service reads `config.yaml` and `policy.txt` from the current directory. Configuration values can be overridden with `IMAGE_GUARD_` environment variables. For example:

```sh
IMAGE_GUARD_SERVER_ADDRESS=:9090 \
IMAGE_GUARD_MODERATION_ENDPOINT=http://127.0.0.1:8081/v1/chat/completions \
go run ./cmd/server
```

SQLite creates `imageguard.db`; uploaded files are stored under `data/images`. Both paths can be changed in `config.yaml`.

Enable debug logging to see the full model prompt (including the policy) and the model's output before decision parsing:

```sh
IMAGE_GUARD_LOGGING_LEVEL=debug go run ./cmd/server
```

These logs include the model name and image metadata; image data is omitted.

Model requests and HTTP response status are logged at the default `info` level. The browser verifies saved verdicts against the API on reload and reconnect; cached results alone do not display an allowed decision.

## API

Upload one JPEG or PNG using multipart field `image`:

```sh
curl -F 'image=@sample.png' http://localhost:8080/v1/images
```

The API returns `202 Accepted` and an Image ID as soon as the file and queued record are stored. Fetch status and the eventual moderation result with:

```sh
curl http://localhost:8080/v1/images/IMAGE_ID
```

Status values are `queued`, `processing`, and `processed`. A processed item contains either a moderation result or a stable processing error code. The uploaded file is deleted after processing; failed file deletions are retried by the worker.

## Alpha limits

The service runs one worker in the API process and uses local disk storage. It returns a deterministic `Yes` or `No` model decision; no confidence score is calculated. Ensure the selected llama.cpp build, ShieldGemma 2 GGUF, vision projector, and chat template work together before relying on moderation results.
