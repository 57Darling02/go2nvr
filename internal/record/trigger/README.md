# Trigger Module Guide

`internal/record/trigger` only decides target recording state.

## Detector contract

```go
Detect(prev, cur *Frame, isRecording bool) bool
```

- Return value is the recorder target state.
- `true` => target is recording.
- `false` => target is stopped/idle.

## Frame input

```go
type Frame struct {
    JPEG       []byte
    JPEGWidth  int
    JPEGHeight int
    Gray       []byte // 64x36
    Width      int    // 64
    Height     int    // 36
    At         time.Time
}
```

- `cur == nil`: no usable new frame this tick.
- `prev == nil`: warm-up stage, no history frame yet.
- `isRecording`: current real recorder state from record layer.

## Best practice (recommended)

Use `simple_diff.go` as the template:

1. Define a file-level schema variable:
   - `var myParams = []DetectorParam{...}`
2. Put `default` and `tip` in that schema.
3. Parse by looping the schema:
   - `parsed := rule.ParseBySchema(myParams)`
4. Register with the same schema:
   - `Register(id, key, name, myParams, NewMyDetector)`

This keeps:

- API metadata output consistent (`default` + `tip`)
- Constructor defaults consistent with metadata
- Contributor onboarding simple (single parameter table)

## DetectorParam fields

```go
type DetectorParam struct {
    Key          string
    Type         string // "number" | "string"
    DefaultValue interface{}
    Min          *int
    Max          *int
    Tip          string
}
```

`Tip` is returned by `/api/record/triggers` and can be rendered by UI.
Rules:

- string param: empty string => default.
- number param: parse failure or out-of-range => default.

## Registration rules

- `id` must be unique (`1` is reserved by `simple_diff`).
- `key` must be unique.
- Duplicate `id/key` registration is ignored with warning logs.

## Runtime lifecycle

1. `record.Init()` creates manager with:
   - `getFrame`, `Start`, `Stop`, `isRecording`
2. Each rule creates one worker (`Apply`).
3. Every `trigger_interval`, manager calls `Detect(...)`.
4. Manager compares detector target with current real status:
   - if different => call `Start` or `Stop`
5. Rule update/delete stops old worker and applies new one.
