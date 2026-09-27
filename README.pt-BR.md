# PickRole

Entre no **AWS IAM Identity Center** (antigo AWS SSO), escolha a conta e o perfil, e tenha credenciais válidas em
**qualquer terminal ou IDE**, sem `export` e sem rodar script em cada shell. Se você usa **CodeArtifact**, o PickRole
também gera o token e atualiza o `settings.xml` do Maven.

Roda no **Linux** (RHEL, Fedora, Ubuntu, Debian e compatíveis) e no **Windows 10 e 11**, com interface em português e
em inglês.

> Este é um resumo. A documentação completa, incluindo arquitetura, decisões e como contribuir, está em inglês, a
> partir do [README principal](README.md).

## Por que

Credenciais do IAM Identity Center costumam ir para variáveis de ambiente, que valem para um terminal só. Abriu outro
terminal, a IDE ou um processo em segundo plano, precisa entrar de novo, e trocar de conta exige refazer tudo em cada
lugar. O PickRole grava as credenciais em `~/.aws/credentials`, que todas as ferramentas da AWS já leem: trocou de
conta no PickRole, todos os terminais usam a nova conta no próximo comando.

## Instalação

Baixe da [página de releases](https://github.com/pickrole/pickrole/releases) e confira os arquivos com o `SHA256SUMS`
da mesma release.

- **RHEL, AlmaLinux e Rocky Linux 8 e 9**: `sudo dnf install ./pickrole_*_el8_x86_64.rpm`.
- **Fedora**: `sudo dnf install ./pickrole_*_fedora_x86_64.rpm`.
- **Ubuntu 22.04+ e Debian 12+**: `sudo apt install ./pickrole_*_amd64.deb`.
- **Windows**: extraia o `pickrole_<versão>_windows_amd64.zip` e rode o `pickrole.exe`. Não precisa instalar nem de
  administrador. Na primeira execução, o Windows pode avisar que o arquivo não é assinado: **Mais informações →
  Executar assim mesmo**.
- **Proxy**: por padrão, o PickRole usa `HTTPS_PROXY`/`HTTP_PROXY` se definidas; senão, o proxy do sistema (no
  Windows, inclusive PAC; no Linux, o proxy manual do GNOME). Para outro proxy, ou nenhum, use **Configurações →
  Rede**, onde **Testar conexão** confere se a AWS responde.

## Primeiro uso

1. Abra o PickRole. Se o seu time já usa, clique em **Importar** e escolha o arquivo de configuração. Se não, informe a
   URL de início do SSO (`https://….awsapps.com/start`) e a região.
2. Autorize no navegador, conferindo o código que o PickRole mostra.
3. Escolha a conta e o perfil. Pronto: qualquer terminal já usa as credenciais.

O idioma segue o do sistema e pode ser trocado em **Configurações → Preferências → Idioma**.
