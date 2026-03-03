package agentbox

import (
	"context"
	"fmt"
)

func newSandboxInfra(opts *SandboxOptions) (*ConnectionConfig, SandboxApi, error) {
	debug := opts.Debug
	config := NewConnectionConfig(&ConnectionConfigOptions{
		APIKey:         opts.APIKey,
		Domain:         opts.Domain,
		Debug:          &debug,
		RequestTimeout: opts.RequestTimeout,
		Proxy:          opts.Proxy,
	})

	sandboxApi, err := NewSandboxApi(config)
	if err != nil {
		return nil, nil, err
	}
	return config, sandboxApi, nil
}

func resolveSandboxSession(
	ctx context.Context,
	opts *SandboxOptions,
	sandboxApi SandboxApi,
	config *ConnectionConfig,
) (sandboxID string, envdVersion string, envdAccessToken string, err error) {
	if config.Debug {
		return "debug_sandbox_id", "", "", nil
	}

	if opts.SandboxID != "" {
		info, err := sandboxApi.GetInfo(ctx, opts.SandboxID)
		if err != nil {
			return "", "", "", err
		}
		sandboxID = info.SandboxID
		envdVersion = info.EnvdVersion
		envdAccessToken = info.EnvdAccessToken
	} else {
		template := opts.Template
		if template == "" {
			template = DefaultTemplate
		}
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = DefaultSandboxTimeout
		}

		info, err := sandboxApi.Create(ctx, &CreateSandboxOptions{
			Template:       template,
			Timeout:        timeout,
			Metadata:       opts.Metadata,
			Envs:           opts.Envs,
			Secure:         opts.Secure,
			APIKey:         opts.APIKey,
			Domain:         opts.Domain,
			Debug:          opts.Debug,
			RequestTimeout: opts.RequestTimeout,
			Proxy:          opts.Proxy,
		})
		if err != nil {
			return "", "", "", err
		}
		sandboxID = info.SandboxID
		envdVersion = info.EnvdVersion
		envdAccessToken = info.EnvdAccessToken
	}

	// Preserve existing headers and append access token when available.
	headers := make(map[string]string)
	for k, v := range config.Headers {
		headers[k] = v
	}
	if envdAccessToken != "" {
		headers["X-Access-Token"] = envdAccessToken
	}
	config.Headers = headers

	return sandboxID, envdVersion, envdAccessToken, nil
}

func buildEnvdAPIURL(config *ConnectionConfig, sandboxID string) string {
	if config.Debug {
		return fmt.Sprintf("http://localhost:%d", EnvdPort)
	}
	return fmt.Sprintf("https://%d-%s.%s", EnvdPort, sandboxID, config.Domain)
}

