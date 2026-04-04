package flipt.authz.v2

import rego.v1

_restricted_envs := {"onoffinc", "default"}

default allow := false

allow if {
	flipt.is_auth_method(input, "oidc")
}

allow if {
	flipt.is_auth_method(input, "token")
	_is_admin_token_request
}

_is_admin_token_request if {
	env := input.request.environment
	ns := input.request.namespace
	_restricted_envs[env]
	ns == input.authentication.metadata.label
}
