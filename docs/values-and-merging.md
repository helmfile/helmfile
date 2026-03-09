# Values Merging and Data Flow

This document explains how Helmfile merges values from various sources and the overall data-flow architecture. Understanding this is essential for writing effective helmfiles.

## Core Architecture Overview

Helmfile processes your configuration in a specific order, with values from different sources being merged together. The key concept is that **later values override earlier values** at the map level (deep merge), while **arrays use smart merging** (sparse auto-detection by default, with CLI overrides using element-by-element merging).

## Values Sources and Precedence

Values in Helmfile come from multiple sources, merged in this order (lowest to highest priority):

```
┌─────────────────────────────────────────────────────────────────┐
│                    VALUES MERGING ORDER                         │
├─────────────────────────────────────────────────────────────────┤
│  1. Base files (from `bases:`)                                  │
│  2. Root-level `values:` block (Defaults)                       │
│  3. Environment values (yaml/yaml.gotmpl)                       │
│  4. Environment values (HCL, including HCL secrets)             │
│  5. Environment secrets (non-HCL, decrypted)                    │
│  6. CLI overrides (--state-values-set, --state-values-file)     │
└─────────────────────────────────────────────────────────────────┘
       ↓ Result: Template-accessible `.Values`
```

**Important:** Non-HCL secrets are loaded and decrypted first, but merged last (step 5), which means they have higher priority than regular environment values and will override duplicate keys.

### 1. Base Files (`bases:`)

Base files are merged first. They allow you to share common configuration across multiple helmfiles:

```yaml
# common.yaml
environments:
  default:
  production:

helmDefaults:
  wait: true
  timeout: 300
```

```yaml
# helmfile.yaml
bases:
  - common.yaml

releases:
  - name: myapp
    chart: mychart
```

### 2. Root-Level `values:` Block

The root-level `values:` block defines default values available to all templates in your helmfile. These are overridden by environment-specific values.

```yaml
# Root-level values - available as {{ .Values.KEY }}
values:
  - appVersion: "1.0.0"
    region: "us-west-2"
    logLevel: "info"

environments:
  production:
    values:
      # This overrides the root-level logLevel
      - logLevel: "warning"

releases:
  - name: myapp
    chart: mychart
    values:
      - image:
          tag: {{ .Values.appVersion }}  # Uses "1.0.0"
        config:
          logLevel: {{ .Values.logLevel }}  # Uses "warning" in production
```

### 3. Environment Values

Environment values are loaded based on the selected environment (`--environment` flag, defaults to `default`):

```yaml
environments:
  default:
    values:
      - env/default.yaml
      - env/common.yaml.gotmpl
  production:
    values:
      - env/production.yaml
      - replicas: 3  # Inline values are also supported
```

### 4. Environment Values Precedence

Within environment values, the precedence is:

1. YAML / YAML.gotmpl files (lowest)
2. HCL files (including HCL secrets)
3. Secrets files - non-HCL only (highest)

**Technical Detail:** 
- **HCL secrets** (.hcl files in `secrets:`) are decrypted and then processed in the HCL loading phase (step 2)
- **Non-HCL secrets** (.yaml/.yaml.gotmpl files in `secrets:`) are decrypted and loaded first, but merged last (step 3), which is why they have the highest priority

```yaml
environments:
  production:
    values:
      - config.yaml        # Loaded first (step 1)
      - config.hcl         # Loaded second (step 2)
    secrets:
      - secrets.yaml       # Merged last (step 3) - highest priority
```

### 5. CLI Overrides

The highest priority values come from CLI flags:

```bash
# Override values from command line
helmfile --state-values-set image.tag=v2.0.0 sync

# Or from a file
helmfile --state-values-file overrides.yaml sync
```

## How Merging Works

### Deep Merge for Maps

Helmfile uses **deep merge** for map values. This means nested maps are merged recursively:

```yaml
# Base values
values:
  - database:
      host: "localhost"
      port: 5432
      credentials:
        username: "admin"

# Environment override
environments:
  production:
    values:
      - database:
          host: "prod-db.example.com"
          credentials:
            password: "secret"  # Added

# Result in production:
# database:
#   host: "prod-db.example.com"    # Overridden
#   port: 5432                      # Preserved from base
#   credentials:
#     username: "admin"             # Preserved from base
#     password: "secret"            # Added from env
```

### Array Merge Strategies

Helmfile uses different array merge strategies depending on the context:

#### 1. Default Strategy (Sparse Auto-Detection)

For most values (state values, environment values), Helmfile uses **sparse auto-detection**:

- **If the override array contains `nil` values** → merge element-by-element
- **If the override array has NO `nil` values** → replace entirely

```yaml
# Example with nil values (sparse array) - merges element-by-element
environments:
  production:
    values:
      - servers:
          - prod1.example.com  # index 0
          - null               # index 1 = nil, triggers merge mode
          - prod3.example.com  # index 2

# Result: merges with base array, preserving index 1 from base
```

```yaml
# Example without nil values (complete array) - replaces entirely
environments:
  production:
    values:
      - servers:
          - prod1.example.com
          - prod2.example.com
          - prod3.example.com

# Result: completely replaces any base array
```

**Important:** An empty array `[]` has no nils, so it **replaces entirely**. This is intentional: explicitly setting an empty array clears the base array.

#### 2. CLI Override Strategy (Element-by-Element Merge)

For `--state-values-set` CLI overrides, arrays are **always merged element-by-element**:

```bash
# This only changes index 0, preserves other indices
helmfile --state-values-set 'servers[0]=prod1.example.com' sync
```

#### 3. Environment Values (Replace by Default)

Within environment values files (yaml/yaml.gotmpl/hcl), arrays use sparse auto-detection (strategy 1 above).

### Practical Array Handling

**Recommendation:** To avoid confusion, treat arrays in one of these ways:

1. **Complete replacement** (default for most cases)
   ```yaml
   # Override array completely
   environments:
     production:
       values:
         - servers: [prod1.example.com, prod2.example.com, prod3.example.com]
   ```

2. **Sparse array merge** (using explicit nil/null values)
   ```yaml
   # Merge specific array indices
   environments:
     production:
       values:
         - servers:
             - prod1.example.com  # index 0: override
             - null                 # index 1: preserve from base
             - prod3.example.com    # index 2: add new
   ```

3. **Use maps instead of arrays** (recommended for complex configurations)
   ```yaml
   # Use maps for better control and merging
   servers:
     server1:
       host: server1.example.com
       enabled: true
     server2:
       host: server2.example.com
       enabled: true
   
   # Environment can add or override specific servers
   environments:
     production:
       values:
         - servers:
             server3:
               host: prod3.example.com
               enabled: true
   ```

## Release-Level Values

Each release can also define its own values, which are **separate** from the helmfile-level values discussed above:

```yaml
# These are STATE values (accessible via {{ .Values }})
values:
  - globalSetting: "value"

environments:
  production:
    values:
      - envSetting: "prod-value"

releases:
  - name: myapp
    chart: mychart
    values:
      # These are RELEASE values (passed to Helm)
      - image:
          tag: {{ .Values.globalSetting }}  # Uses state value
      - values.yaml.gotmpl                  # Template file using state values
      - config.yaml                         # Static file
```

### Release Values Merging

Within a release, values from different sources are merged in this order:

1. Values from release template (`templates:` with `values:`)
2. Inline values in the release
3. Values files listed in release
4. Values from `valuesTemplate:`
5. `set` and `setString` values

```yaml
templates:
  myTemplate:
    values:
      - templateDefaults:
          option: "a"  # Lowest priority for this release

releases:
  - name: myapp
    chart: mychart
    inherit:
      - template: myTemplate
    values:
      - releaseDefaults:
          option: "b"    # Overrides template
      - env-specific.yaml # Can override above
    set:
      - name: option
        value: "c"       # Highest priority
```

## Data Flow Diagram

Here's the complete data flow when running `helmfile sync`:

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                         HELMFILE DATA FLOW                                  │
└─────────────────────────────────────────────────────────────────────────────────────┘

1. INITIALIZATION PHASE (app.go, desired_state_file_loader.go)
   ┌──────────────────┐
   │ Parse CLI flags │ ──> --state-values-set / --state-values-file
   └──────────────────┘
           │
           ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │ Create Environment {                                               │
   │   Name: "default" or --environment value                        │
   │   Defaults: {} (will hold root-level values:)                  │
   │   Values: {} (will hold environment values + secrets)          │
   │   CLIOverrides: <parsed CLI values> (highest priority)    │
   │ }                                                                   │
   └──────────────────────────────────────────────────────────────────────────┘

2. LOAD PHASE (for each helmfile.yaml) (create.go: LoadEnvValues)
   ┌──────────────┐
   │ bases:        │ ──> Load and merge base files (if any)
   └──────────────┘
           │
           ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │ values: (root-level)│ ──> Load via loadValuesEntries()                            │
   │                    │     Store in Environment.Defaults                              │
   └──────────────────────────────────────────────────────────────────────────┘
           │
           ▼
   ┌──────────────────────────────────────────────────────────────────────────┐
   │ environments:      │ ──> loadEnvValues() processes in order:                   │
   │                      │     1. Decrypt all secrets (HCL and non-HCL)                  │
   │                      │     2. Load environment values:                                 │
   │                      │        - yaml/yaml.gotmpl files (merged)                        │
   │                      │        - HCL files incl. decrypted HCL (merged)            │
   │                      │     3. Merge non-HCL decrypted secrets (highest priority)  │
   │                      │     Store result in Environment.Values                          │
   └──────────────────────────────────────────────────────────────────────────┘
           │
           ▼
   ┌───────────────────────────────────────────────────────────────────┐
   │ Update Environment from ctxEnv and overrodeEnv:                     │
   │   - Merge ctxEnv values (for multi-part helmfiles)                 │
   │   - Merge overrodeEnv (CLIOverrides already included here)         │
   └───────────────────────────────────────────────────────────────────┘

3. FINAL MERGE PHASE (environment.go: GetMergedValues)
   ┌───────────────────────────────────────────────────────────────────┐
   │ GetMergedValues() called when accessing .Values in templates:      │
   │   result = {}                                                        │
   │   result = merge(result, Defaults)      # Root-level values:        │
   │   result = merge(result, Values)        # Environment values+secrets│
   │   result = merge(result, CLIOverrides)  # CLI flags (highest priority)│
   │                                                                     │
   │ Special: CLIOverrides uses ArrayMergeStrategyMerge (element-by-element)│
   │         Defaults and Values use ArrayMergeStrategySparse (auto-detect) │
   └───────────────────────────────────────────────────────────────────┘
           │
           ▼
   ┌───────────────────────────────────────────────────────────────────┐
   │ state.RenderedValues = result                                    │
   │ Accessible via {{ .Values.KEY }} in all templates                 │
   └───────────────────────────────────────────────────────────────────┘

4. TEMPLATE RENDERING PHASE
   ┌──────────────────────────────────────────────┐
   │ Render helmfile.yaml templates             │
   │ Render .gotmpl values files                │
   │ Prepare release-specific values            │
   └──────────────────────────────────────────────┘

5. HELM EXECUTION PHASE
   ┌──────────────────────────────────────────────┐
   │ helm upgrade --install RELEASE CHART \       │
   │   -f /tmp/generated-values-xxx.yaml \        │
   │   --set key=value                            │
   └──────────────────────────────────────────────┘
```

**Key Implementation Details:**

1. **CLI Overrides** are parsed first (Load phase) and merged last (GetMergedValues)
2. **Root-level `values:`** are loaded into `Environment.Defaults` (not Values!)
3. **Environment values + secrets** are loaded into `Environment.Values`
4. **Final merge order** in `GetMergedValues()`:
   - Defaults → Values → CLIOverrides
5. **Array merge strategies**:
   - Defaults & Values: `ArrayMergeStrategySparse` (auto-detect nil values)
   - CLIOverrides: `ArrayMergeStrategyMerge` (always element-by-element)

## Technical Details

### Secret Handling Intern

Helmfile processes secrets in a special way:

**Non-HCL secrets (.yaml, .yaml.gotmpl):**
1. Decrypted using helm-secrets plugin
2. Parsed into values immediately (during load phase)
3. Stored separately from regular values
4. **Mrged last** (highest priority) after all environment values are loaded

**HCL secrets (.hcl):**
1. Decrypted using helm-secrets plugin
2. Decrypted file paths added to values file list
3. Processed in step 2 (HCL loading phase)
4. Can reference values from other HCL files (using `hv.` accessor)

This separation allows:
- Secrets to override regular values without being re-decrypted multiple times
- HCL secrets to participate in HCL's cross-file referencing system

### Multiple Helmfiles and State files

When using multi-part helmfiles (multiple YAML documents separated by `---`):

```yaml
# Part 1: base.yaml
helmDefaults:
  wait: true
  timeout: 300
---
# Part 2: environments.yaml
environments:
  default:
  production:
```

Each part is processed in order:
 and the results are merged with later parts taking precedence.

## Technical Details

### Environment Structure Internals

The `Environment` struct has three key fields that affect merging:

```go
type Environment struct {
    Name         string
    KubeContext  string
    Values       map[string]any  // Environment values + secrets
    Defaults     map[string]any  // Root-level values: block
    CLIOverrides map[string]any  // CLI --state-values-set
}
```

### Final Merge Process (GetMergedValues)

When you access `.Values` in templates, Helmfile calls `GetMergedValues()` which merges in this order:

```go
func (e *Environment) GetMergedValues() (map[string]any, error) {
    vals := map[string]any{}
    vals = maputil.MergeMaps(vals, e.Defaults)     // 1. Defaults (root-level values:)
    vals = maputil.MergeMaps(vals, e.Values)        // 2. Values (environment values + secrets)
    vals = maputil.MergeMaps(vals, e.CLIOverrides, // 3. CLIOverrides (highest priority)
        maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})
    return vals, nil
}
```

**Important:** CLIOverrides uses `ArrayMergeStrategyMerge` (element-by-element merging), while Defaults and Values use the default strategy (sparse auto-detection).

### Merging Library: mergo

Helmfile uses the [mergo](https://github.com/imdario/mergo) library for deep merging with these key features:

1. **Deep merge for maps**: Nested maps are merged recursively
2. **WithOverride option**: Later values override earlier values
3. **Type-safe**: Preserves value types during merge

Example from code:
```go
// In loadEnvValues()
if err := mergo.Merge(&valuesVals, &secretVals, mergo.WithOverride); err != nil {
    return nil, err
}
```

### Array Merge Strategies Implementation

The `maputil.MergeMaps` function supports three array merge strategies:

```go
type ArrayMergeStrategy int

const (
    ArrayMergeStrategySparse  ArrayMergeStrategy = iota  // Auto-detect based on nil values
    ArrayMergeStrategyReplace ArrayMergeStrategy = iota  // Always replace arrays
    ArrayMergeStrategyMerge   ArrayMergeStrategy = iota  // Always merge element-by-element
)
```

**Sparse Strategy (Default for most cases):**
```go
func mergeSlices(base, override []any, strategy ArrayMergeStrategy) []any {
    if strategy == ArrayMergeStrategySparse {
        isSparse := false
        for _, v := range override {
            if v == nil {
                isSparse = true
                break
            }
        }
        if !isSparse {
            return override  // Replace entirely
        }
        // Otherwise merge element-by-element
    }
}
```

This means:
- `[1, 2, 3]` merged with `[4, 5]` → result: `[4, 5]` (replaced, no nils)
- `[null, 2]` merged with `[1, 2, 3]` → result: `[1, 2, 3]` (merged, has nil)

## Common Patterns

### Pattern 1: Global Defaults with Environment Overrides

```yaml
# Global defaults
values:
  - replicas: 1
    logLevel: "debug"
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"

# Environment-specific overrides
environments:
  production:
    values:
      - replicas: 3
        logLevel: "warning"
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
```

### Pattern 2: Shared Values Across Releases

```yaml
values:
  - registry: "docker.io"
    imageTag: "latest"

releases:
  - name: frontend
    chart: charts/frontend
    values:
      - image:
          repository: {{ .Values.registry }}/frontend
          tag: {{ .Values.imageTag }}

  - name: backend
    chart: charts/backend
    values:
      - image:
          repository: {{ .Values.registry }}/backend
          tag: {{ .Values.imageTag }}
```

### Pattern 3: Complex Nested Merging

```yaml
values:
  - monitoring:
      enabled: true
      endpoints:
        health: "/health"
        metrics: "/metrics"
      alerts:
        email:
          enabled: true
          recipients:
            - ops@example.com

environments:
  production:
    values:
      - monitoring:
          alerts:
            slack:
              enabled: true
              channel: "#alerts"
            email:
              recipients:
                - ops@example.com
                - oncall@example.com

# Result in production:
# monitoring:
#   enabled: true
#   endpoints:
#     health: "/health"
#     metrics: "/metrics"
#   alerts:
#     email:
#       enabled: true
#       recipients: [ops@example.com, oncall@example.com]  # REPLACED!
#     slack:
#       enabled: true
#       channel: "#alerts"
```

## Technical Implementation Details

### Environment Structure

The `Environment` struct is the core data structure that holds all values:

```go
type Environment struct {
    Name         string
    KubeContext  string
    Defaults     map[string]any  // Root-level values: block (loaded via loadValuesEntries)
    Values       map[string]any  // Environment values + secrets (loaded in loadEnvValues)
    CLIOverrides map[string]any  // CLI --state-values-set (parsed in Load phase)
}
```

**Key Insight:** These three fields are kept **separate** until the final merge in `GetMergedValues()`.

### Merge Process Details

#### 1. Loading Root-Level `values:` (create.go:172-179)

```go
// In LoadEnvValues():
newDefaults, err := state.loadValuesEntries(nil, state.DefaultValues, c.remote, ctxEnv, env)
if err := nil {
    return nil, err
}

if err := mergo.Merge(&e.Defaults, newDefaults, mergo.WithOverride); err != nil {
    return nil, err
}
```

- Root-level `values:` block is loaded into `Environment.Defaults`
- Uses `mergo.Merge` with `mergo.WithOverride` option
- This happens **after** environment values are loaded (line 172 comes after line 167)

#### 2. Loading Environment Values (create.go:385-439)

```go
// In loadEnvValues:
// Step 1: Decrypt non-HCL secrets first
decryptedFiles, err := c.scatterGatherEnvSecretFiles(st, envSecretFiles, secretVals, keepSecretFilesExtensions)

// Step 2: Load environment values (yaml/gotmpl + HCL + non-HCL secrets)
envValuesEntries := append(decryptedFiles, envSpec.Values...)  // Non-HCL secrets first, then env values
 valuesVals, err = st.loadValuesEntries(envSpec.MissingFileHandler, envValuesEntries, c.remote, loadValuesEntriesEnv, name)

// Step 3: Merge secrets into valuesVals (highest priority among env values)
if err = mergo.Merge(&valuesVals, &secretVals, mergo.WithOverride); err != nil {
    return nil, err
}

// Step 4: Create Environment with merged values
newEnv := &environment.Environment{
    Name: name, 
    Values: valuesVals,  // Contains: env values + HCL secrets + non-HCL secrets
    Defaults: map[string]any{}, 
    CLIOverrides: map[string]any{},
}

// Step 5: Merge from ctxEnv and overrodeEnv (for multi-part helmfiles and CLI overrides)
if ctxEnv != nil {
    newEnv.Defaults = maputil.MergeMaps(ctxEnv.Defaults, newEnv.Defaults)
    newEnv.Values = maputil.MergeMaps(ctxEnv.Values, newEnv.Values)
    newEnv.CLIOverrides = maputil.MergeMaps(newEnv.CLIOverrides, ctxEnv.CLIOverrides,
        maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})
}
if overrode != nil {
    newEnv.Defaults = maputil.MergeMaps(newEnv.Defaults, overrode.Defaults)
    newEnv.Values = maputil.MergeMaps(newEnv.Values, overrode.Values)
    newEnv.CLIOverrides = maputil.MergeMaps(newEnv.CLIOverrides, overrode.CLIOverrides,
        maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})
}
```

**Key Points:**
1. Non-HCL secrets are decrypted **first**, but merged **last** (highest priority)
2. HCL secrets are processed with HCL loader (can reference other HCL values)
3. CLI overrides come from `overrodeEnv` parameter
4. Multiple merge operations use `mergo.Merge` with `mergo.WithOverride`

#### 3. Final Merge (environment.go:115-129)

```go
// Called when accessing .Values in templates:
func (e *Environment) GetMergedValues() (map[string]any, error) {
    vals := map[string]any{}
    vals = maputil.MergeMaps(vals, e.Defaults)      // 1. Merge Defaults
    vals = maputil.MergeMaps(vals, e.Values)        // 2. Merge Values
    vals = maputil.MergeMaps(vals, e.CLIOverrides,  // 3. Merge CLIOverrides
        maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})
    
    vals, err := maputil.CastKeysToStrings(vals)
    if err != nil {
        return nil, err
    }
    return vals, nil
}
```

**Key Points:**
1. **Defaults** (root-level values:) are merged first (lowest priority)
2. **Values** (environment values + secrets) are merged second
3. **CLIOverrides** are merged last (highest priority)
4. **CLIOverrides** always use `ArrayMergeStrategyMerge` (element-by-element)
5. **Defaults/Values** use `ArrayMergeStrategySparse` (auto-detect nil values)

### Deep Merge Behavior

Helmfile uses the `dario.cat/mergo` library with `WithOverride` option:

**Maps:**
```go
if err := mergo.Merge(&dest, &src, mergo.WithOverride); err != nil {
    // handle error
}
```

This performs:
1. **Deep merge**: Nested maps are merged recursively
2. **Override**: Values from `src` override values in `dest`
3. **Type preservation**: Types are preserved during merge

**Arrays (via maputil.MergeMaps):**
```go
func MergeMaps(a, b map[string]interface{}, opts ...MergeOptions) map[string]interface{} {
    arrayStrategy := ArrayMergeStrategySparse  // default
    
    // ... merging logic ...
    
    if vSlice != nil {  // Array detected
        if outSlice != nil {
            out[k] = mergeSlices(outSlice, vSlice, arrayStrategy)
            continue
        }
    }
}
```

**Three Array Merge Strategies:**

1. **ArrayMergeStrategySparse** (default for Defaults & Values)
   - Auto-detects based on nil values in override array
   - If **any nil values**: merge element-by-element (sparse)
   - If **no nil values**: replace entire array (complete)

2. **ArrayMergeStrategyReplace** (not used in standard flow)
   - Always replaces entire array
   - Used for complete array replacement

3. **ArrayMergeStrategyMerge** (used for CLIOverrides)
   - Always merges element-by-element
   - Preserves base array elements unless explicitly overridden
   - Perfect for CLI `array[index]=value` syntax

### Loading Order vs Merge Order

**Important Distinction:**

| Values Source    | Loading Order        | Merge Priority |
|------------------|----------------------|----------------|
| CLI Overrides    | Loaded **first** (init) | Merged **last** (highest)  |
| Root values:     | Loaded **second**       | Merged **first** (lowest) |
| Environment values| Loaded **third**       | Merged **second**        |
| Environment secrets| Loaded **fourth**      | Merged **second** (with values)|

**Why this matters:**
- CLI overrides are available early (for template rendering)
- But they don't override until the final merge
- This allows environment values to reference CLI overrides in templates
- But final merge ensures CLI values win

## Key Takeaways

1. **Values merge deeply** - nested maps are merged recursively, not replaced
2. **Arrays use smart merging** - by default, complete arrays replace, sparse arrays merge element-by-element
3. **Later values win** - the order of sources determines precedence
4. **Separate concerns** - state values (`.Values`) vs release values (passed to Helm)
5. **Use templates** - avoid repetition by using `templates:` and root-level `values:`
6. **Secrets have high priority** - non-HCL secrets are merged last, overriding environment values
7. **Prefer maps over arrays** - for configuration that needs incremental updates, use maps instead of arrays
8. **CLI overrides are special** - loaded first but merged last, always using element-by-element array merging

## Troubleshooting

### Value not being overridden

Check the precedence order. A value defined in a base file might be overridden by environment values. Remember that **non-HCL secrets have the highest priority** among environment values.

### Unexpected array behavior

Arrays use **sparse auto-detection** by default:
- Arrays with `nil` values: merged element-by-element
- Arrays without `nil` values: **replaced entirely**
- Empty arrays `[]`: **replace entirely** (clears the base array)

To merge arrays, either:
1. Use `null` (nil) values for indices you want to preserve from the base
2. Convert to maps for better control
3. Use `--state-values-set` CLI flags for element-by-element updates

### Can't access value in template

Ensure the value is defined in **state values** (not just in release values). State values are accessible via `{{ .Values.KEY }}`, while release values are only passed to Helm.

### Values file not being templated

Only files with `.gotmpl` extension are templated. Regular `.yaml` files are used verbatim.

### Secrets not overriding values

Non-HCL secrets are merged **last** in the environment values loading process, giving them the highest priority among environment values. If your secrets aren't overriding, check:
1. The secrets file is properly encrypted
2. The secrets are listed in the `secrets:` section (not `values:`)
3. You're using the correct helm-secrets plugin

### CLI overrides not working

CLI `--state-values-set` uses **element-by-element array merging**, which is different from file-based values. This means:
```bash
# This merges at index 0, preserving other indices
helmfile --state-values-set 'servers[0]=prod1.example.com' sync

# This replaces the entire array
helmfile --state-values-file <(echo 'servers: [prod1.example.com]') sync
```
