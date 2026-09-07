package com.cooper.{{package_name}}.controller;

import com.cooper.{{package_name}}.model.HealthResponse;
import com.cooper.{{package_name}}.service.HealthService;

public class HealthController {
    private final HealthService healthService;

    public HealthController(HealthService healthService) {
        this.healthService = healthService;
    }

    public HealthResponse getHealth() {
        return healthService.checkHealth();
    }
}
