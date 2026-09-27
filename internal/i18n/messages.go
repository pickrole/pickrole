package i18n

// catalog holds every message the backend shows to the user. Keys are
// "package.message". Each language must have every English key (see the
// test); %s/%q/%d follow fmt.
var catalog = map[Lang]map[string]string{
	English: { // #nosec G101 -- messages that mention credentials and tokens, not secrets
		"app.configure_sso_first":     "Set up the SSO connection first",
		"app.config_too_large":        "The configuration file is too large (1 MB max)",
		"app.unexpected_verification": "Unexpected authorization address: %q",
		"app.no_login_in_progress":    "No sign-in in progress",
		"app.saving_cache":            "Couldn't save the account cache",
		"app.account_not_found":       "Account not found. Refresh the account list.",
		"app.role_not_found":          "This role isn't available for the account. Refresh the account list.",
		"app.writing_credentials":     "Couldn't write the credentials",
		"app.load_profile_first":      "Load a profile in this PickRole session first",
		"app.history_not_saved":       "Couldn't save the history",
		"app.import_title":            "Import PickRole configuration",
		"app.export_title":            "Export PickRole configuration",
		"app.json_filter":             "PickRole configuration (*.json)",

		"awsfiles.invalid_credential": "AWS returned an invalid credential (%s)",
		"awsfiles.invalid_profile":    "Invalid profile name: %q",

		"clipboard.unsupported": "Protected copy isn't supported on this system",

		"codeartifact.no_access": "This profile has no access to CodeArtifact",
		"codeartifact.token":     "Couldn't get the CodeArtifact token",

		"config.reading":             "Couldn't read %s",
		"config.invalid_file":        "Invalid configuration file",
		"config.start_url":           "Enter the SSO start URL (https://…/start)",
		"config.sso_region":          "Invalid SSO region: %q",
		"config.profile_mode":        "Unknown profile mode: %q",
		"config.prod_pattern":        "Invalid production pattern",
		"config.language":            "Unknown language: %q",
		"config.ca_domain":           "Enter the CodeArtifact domain",
		"config.ca_owner":            "The domain owner account must have 12 digits",
		"config.ca_region":           "Invalid CodeArtifact region: %q",
		"config.maven_server_id":     "The Maven server ID accepts only letters, digits, dots, hyphens and underscores",
		"config.maven_settings_path": "Maven's settings.xml must be inside your home folder and end in .xml",
		"config.proxy_mode":          "Unknown proxy mode: %q",
		"config.proxy_url":           "Enter the proxy address as http://host:port (https:// and socks5:// work too)",
		"config.proxy_credentials":   "Don't put a user or password in the proxy address; for a proxy that needs them, use the HTTPS_PROXY variable",
		"config.no_proxy":            "Invalid proxy exception: %q",

		"proxy.pac_unsupported": "The system uses an automatic proxy configuration (PAC), which PickRole doesn't read on Linux. Set the proxy manually.",
		"proxy.unreachable":     "Couldn't reach %s",

		"maven.invalid_settings": "settings.xml has no </settings>, so it can't be edited safely",
		"maven.empty_server_id":  "Empty Maven server ID",
		"maven.backup":           "Couldn't create the backup",

		"sso.login_required":      "Your SSO session expired. Sign in again.",
		"sso.register_client":     "Couldn't register the OIDC client",
		"sso.start_authorization": "Couldn't start the authorization",
		"sso.not_authorized":      "Authorization wasn't completed",
		"sso.get_token":           "Couldn't get the token",
		"sso.no_credentials":      "AWS returned no credentials for this role",
	},
	Portuguese: { // #nosec G101 -- messages that mention credentials and tokens, not secrets
		"app.configure_sso_first":     "Configure a conexão SSO primeiro",
		"app.config_too_large":        "O arquivo de configuração é grande demais (máximo 1 MB)",
		"app.unexpected_verification": "Endereço de autorização inesperado: %q",
		"app.no_login_in_progress":    "Nenhum login em andamento",
		"app.saving_cache":            "Não foi possível salvar o cache de contas",
		"app.account_not_found":       "Conta não encontrada. Atualize a lista de contas.",
		"app.role_not_found":          "Este perfil não está disponível na conta. Atualize a lista de contas.",
		"app.writing_credentials":     "Não foi possível gravar as credenciais",
		"app.load_profile_first":      "Carregue um perfil nesta sessão do PickRole primeiro",
		"app.history_not_saved":       "Não foi possível salvar o histórico",
		"app.import_title":            "Importar configuração do PickRole",
		"app.export_title":            "Exportar configuração do PickRole",
		"app.json_filter":             "Configuração do PickRole (*.json)",

		"awsfiles.invalid_credential": "A AWS devolveu uma credencial inválida (%s)",
		"awsfiles.invalid_profile":    "Nome de perfil inválido: %q",

		"clipboard.unsupported": "Cópia protegida não é suportada neste sistema",

		"codeartifact.no_access": "Este perfil não tem acesso ao CodeArtifact",
		"codeartifact.token":     "Não foi possível obter o token do CodeArtifact",

		"config.reading":             "Não foi possível ler %s",
		"config.invalid_file":        "Arquivo de configuração inválido",
		"config.start_url":           "Informe a URL de início do SSO (https://…/start)",
		"config.sso_region":          "Região do SSO inválida: %q",
		"config.profile_mode":        "Modo de perfil desconhecido: %q",
		"config.prod_pattern":        "Padrão de produção inválido",
		"config.language":            "Idioma desconhecido: %q",
		"config.ca_domain":           "Informe o domínio do CodeArtifact",
		"config.ca_owner":            "A conta dona do domínio deve ter 12 dígitos",
		"config.ca_region":           "Região do CodeArtifact inválida: %q",
		"config.maven_server_id":     "O ID do server do Maven aceita só letras, números, ponto, hífen e sublinhado",
		"config.maven_settings_path": "O settings.xml do Maven precisa ficar na sua pasta de usuário e terminar em .xml",
		"config.proxy_mode":          "Modo de proxy desconhecido: %q",
		"config.proxy_url":           "Informe o endereço do proxy como http://host:porta (https:// e socks5:// também funcionam)",
		"config.proxy_credentials":   "Não coloque usuário ou senha no endereço do proxy; para um proxy que exige isso, use a variável HTTPS_PROXY",
		"config.no_proxy":            "Exceção de proxy inválida: %q",

		"proxy.pac_unsupported": "O sistema usa configuração automática de proxy (PAC), que o PickRole não lê no Linux. Configure o proxy manualmente.",
		"proxy.unreachable":     "Não foi possível alcançar %s",

		"maven.invalid_settings": "O settings.xml não tem </settings>, então não dá para editá-lo com segurança",
		"maven.empty_server_id":  "ID do server do Maven vazio",
		"maven.backup":           "Não foi possível criar o backup",

		"sso.login_required":      "Sua sessão SSO expirou. Entre novamente.",
		"sso.register_client":     "Não foi possível registrar o cliente OIDC",
		"sso.start_authorization": "Não foi possível iniciar a autorização",
		"sso.not_authorized":      "A autorização não foi concluída",
		"sso.get_token":           "Não foi possível obter o token",
		"sso.no_credentials":      "A AWS não devolveu credenciais para este perfil",
	},
}
