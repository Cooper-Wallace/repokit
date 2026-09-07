package com.cooper.{{package_name}}.service;

import com.cooper.{{package_name}}.model.HealthResponse;

public class HealthService {
    public HealthResponse checkHealth() {
        return new HealthResponse("UP", "{{repo_name}}");
    }
}
