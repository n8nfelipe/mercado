# Spec Delta

## Purpose

Define como pessoas criam conta, autenticam-se, mantêm sessão e recebem papéis, além do onboarding necessário para atuar como vendedor com segurança.

## ADDED Requirements

### Requirement: Cadastro com email e senha

O sistema SHALL permitir que qualquer visitante crie uma conta informando nome, email e senha. O email MUST ser normalizado para minúsculas, MUST ser único e a senha MUST ter no mínimo 12 caracteres. Contas criadas por esse fluxo recebem o papel `buyer`.

#### Scenario: Cadastro válido
- **WHEN** um visitante envia nome, email inédito e senha com 12 ou mais caracteres
- **THEN** o sistema cria a conta com papel `buyer` e responde com o usuário criado, sem retornar a senha

#### Scenario: Email duplicado
- **WHEN** um visitante tenta cadastrar um email já existente, ignorando diferenças de maiúsculas e minúsculas
- **THEN** o sistema responde `409` com código `email_already_registered`

#### Scenario: Senha fraca
- **WHEN** um visitante envia senha com menos de 12 caracteres
- **THEN** o sistema responde `400` com erro de validação no campo `password`

#### Scenario: Email inválido
- **WHEN** um visitante envia email sem formato válido
- **THEN** o sistema responde `400` com erro de validação no campo `email`

### Requirement: Armazenamento seguro de credenciais

O sistema SHALL armazenar senhas apenas como hash Argon2id com salt único por usuário. Senhas em texto puro, hashes e tokens de sessão MUST NOT aparecer em logs, respostas ou métricas.

#### Scenario: Senha não é persistida em texto puro
- **WHEN** uma conta é criada
- **THEN** a coluna de senha contém apenas o hash Argon2id e a senha original não é recuperável pela aplicação

#### Scenario: Dados sensíveis fora dos logs
- **WHEN** cadastro, login ou logout são executados
- **THEN** os logs da requisição não contêm senha, hash nem valor do cookie de sessão

### Requirement: Login e sessão

O sistema SHALL autenticar credenciais válidas e criar uma sessão com prazo de expiração. A sessão MUST ser transportada em cookie `HttpOnly`, `Secure` quando a conexão for HTTPS e `SameSite=Lax`. Um novo identificador de sessão MUST ser emitido a cada login bem-sucedido.

#### Scenario: Login válido
- **WHEN** um usuário cadastrado envia email e senha corretos
- **THEN** o sistema responde `200`, emite o cookie de sessão e retorna os dados públicos do usuário

#### Scenario: Credenciais inválidas
- **WHEN** um usuário envia senha incorreta ou email inexistente
- **THEN** o sistema responde `401` com a mesma mensagem genérica para os dois casos

#### Scenario: Sessão expirada
- **WHEN** uma requisição autenticada apresenta sessão expirada ou inexistente
- **THEN** o sistema responde `401` com código `unauthenticated`

#### Scenario: Renovação do identificador de sessão
- **WHEN** o mesmo usuário faz login duas vezes
- **THEN** cada login recebe um identificador de sessão diferente

### Requirement: Logout

O sistema SHALL invalidar a sessão atual quando o usuário solicitar logout, removendo o cookie e impedindo a reutilização do identificador de sessão.

#### Scenario: Logout invalida a sessão
- **WHEN** um usuário autenticado chama logout e depois tenta reutilizar o cookie anterior
- **THEN** o sistema responde `204` no logout e `401` na tentativa seguinte

### Requirement: Papéis e autorização por recurso

O sistema SHALL atribuir papéis `buyer`, `seller` e `admin` e MUST verificar autorização no backend em toda rota protegida. Recursos de um usuário MUST NOT ser acessíveis por outro usuário, exceto quando a rota for explicitamente administrativa.

#### Scenario: Acesso a recurso próprio
- **WHEN** um vendedor consulta um anúncio do qual é proprietário
- **THEN** o sistema permite a operação

#### Scenario: Acesso a recurso de terceiro
- **WHEN** um vendedor tenta editar um anúncio de outro vendedor
- **THEN** o sistema responde `403` e não altera o anúncio

#### Scenario: Rota administrativa
- **WHEN** um usuário sem papel `admin` chama uma rota administrativa
- **THEN** o sistema responde `403` com código `forbidden`

### Requirement: Onboarding de vendedor

O sistema SHALL permitir que um usuário autenticado se torne vendedor informando nome de exibição, documento e contato. O documento MUST ser único entre vendedores e MUST NOT ser exposto em respostas públicas. A conclusão do onboarding adiciona o papel `seller`.

#### Scenario: Onboarding concluído
- **WHEN** um comprador autenticado envia dados válidos de vendedor
- **THEN** o sistema cria o perfil, adiciona o papel `seller` e retorna o perfil público sem o documento

#### Scenario: Documento duplicado
- **WHEN** o documento informado já pertence a outro vendedor
- **THEN** o sistema responde `409` com código `document_already_registered`

#### Scenario: Dados obrigatórios ausentes
- **WHEN** o onboarding é enviado sem nome de exibição, documento ou contato
- **THEN** o sistema responde `400` com erros dos campos ausentes

### Requirement: Limite de tentativas de login

O sistema SHALL limitar tentativas de login por combinação de conta e endereço de origem. Ao exceder o limite, MUST responder `429` com `Retry-After` e MUST NOT revelar se a conta existe.

#### Scenario: Excesso de tentativas
- **WHEN** ocorrem tentativas de login acima do limite configurado para a mesma conta e origem
- **THEN** o sistema responde `429` sem informar se o email está cadastrado
