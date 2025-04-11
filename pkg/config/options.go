package config

func (o *ApplyOptions) HandleFlag(name string, value interface{}, changed bool) {
    if !changed {
        return
    }

    switch name {
    case "include-crds":
        if boolVal, ok := value.(*bool); ok {
            o.IncludeCRDsFlag.Set(*boolVal)
        }
    case "skip-crds":
        if boolVal, ok := value.(*bool); ok {
            o.SkipCRDsFlag.Set(*boolVal)
        }
    // Handle other flags...
    }
}

func (o *DiffOptions) HandleFlag(name string, value interface{}, changed bool) {
    if !changed {
        return
    }

    switch name {
    case "include-crds":
        if boolVal, ok := value.(*bool); ok {
            o.IncludeCRDsFlag.Set(*boolVal)
        }
    // Handle other flags...
    }
}

func (o *SyncOptions) HandleFlag(name string, value interface{}, changed bool) {
    if !changed {
        return
    }

    switch name {
    case "include-crds":
        if boolVal, ok := value.(*bool); ok {
            o.IncludeCRDsFlag.Set(*boolVal)
        }
    case "skip-crds":
        if boolVal, ok := value.(*bool); ok {
            o.SkipCRDsFlag.Set(*boolVal)
        }
    // Handle other flags...
    }
}

func (o *TemplateOptions) HandleFlag(name string, value interface{}, changed bool) {
    if !changed {
        return
    }

    switch name {
    case "include-crds":
        if boolVal, ok := value.(*bool); ok {
            o.IncludeCRDsFlag.Set(*boolVal)
        }
    // Handle other flags...
    }
}