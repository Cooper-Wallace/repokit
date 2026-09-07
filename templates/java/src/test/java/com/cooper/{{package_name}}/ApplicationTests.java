package com.cooper.{{package_name}};

import com.cooper.{{package_name}}.model.HealthResponse;
import com.cooper.{{package_name}}.service.HealthService;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class ApplicationTests {

    @Test
    void healthCheckShouldReturnUp() {
        HealthService service = new HealthService();
        HealthResponse response = service.checkHealth();
        assertNotNull(response);
        assertEquals("UP", response.status());
        assertEquals("{{repo_name}}", response.service());
    }
}
