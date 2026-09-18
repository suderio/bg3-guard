# Walkthrough: Auto-Spawn no Windows Terminal e Duplo-Clique

Implementamos com sucesso a rotina nativa de auto-spawn no Windows para que o `bg3-guard.exe` possa ser aberto via duplo-clique no Windows Explorer, iniciando automaticamente dentro do **Windows Terminal (`wt.exe`)** (ou com fallback para `cmd.exe`), sem exibir a mensagem do Mousetrap do Cobra.

## Mudanças Realizadas

### 1. Pacote `internal/terminal`

- [terminal.go](file:///d:/repos/bg3-guard/internal/terminal/terminal.go): Define a constante de controle `EnvTerminalSpawned = "BG3_GUARD_TERMINAL"`.
- [terminal_windows.go](file:///d:/repos/bg3-guard/internal/terminal/terminal_windows.go):
  - Utiliza `kernel32.dll` e a API Win32 `GetConsoleProcessList` para verificar a quantidade de processos vinculados ao console atual.
  - Se `GetConsoleProcessList` retornar `1`, identifica que o aplicativo foi iniciado por duplo-clique no Explorer (que aloca um console individual com apenas o processo atual).
  - Se a variável de controle `BG3_GUARD_TERMINAL=1` estiver presente, aborta a rotina para impedir auto-reinicializações em cascata.
  - **Prioridade 1 (`wt.exe`)**: Busca `wt.exe` via `exec.LookPath`. Quando encontrado, executa `wt.exe -w new <exe> [args...]` com a variável `BG3_GUARD_TERMINAL=1` e fecha o processo inicial via `os.Exit(0)`.
  - **Fallback (`cmd.exe`)**: Caso `wt.exe` não esteja disponível, busca `cmd.exe` e executa via `cmd.exe /c start "" <exe> [args...]` com `BG3_GUARD_TERMINAL=1` e encerra o processo inicial via `os.Exit(0)`.
  - Se ambos falharem, o programa continua sua execução no console atual sem falhar.
- [terminal_other.go](file:///d:/repos/bg3-guard/internal/terminal/terminal_other.go):
  - Tag `//go:build !windows` com implementação no-op de `EnsureInteractiveTerminal()`, preservando a compatibilidade de compilação cruzada no Linux e macOS.
- [terminal_test.go](file:///d:/repos/bg3-guard/internal/terminal/terminal_test.go) e [terminal_windows_test.go](file:///d:/repos/bg3-guard/internal/terminal/terminal_windows_test.go):
  - Testes unitários para validar a detecção de console, respeito à variável de ambiente e execução segura em sessões de teste.

### 2. Integração no Cobra CLI

- [cmd/root.go](file:///d:/repos/bg3-guard/cmd/root.go):
  - Em `init()`: define `cobra.MousetrapHelpText = ""` para desativar a tela informativa do Cobra ao ser invocado pelo Explorer.
  - Em `Execute()`: invoca `terminal.EnsureInteractiveTerminal()` antes de executar a árvore de comandos do Cobra.

---

## Verificação e Testes

### 1. Testes Unitários
```powershell
go test -v ./internal/terminal/...
go test -v ./...
```
- **Resultado**: Todos os testes passaram com sucesso (`PASS`).

### 2. Compilação Multiplataforma (Linux e macOS)
```powershell
$env:GOOS='linux'; go test -c ./internal/terminal; go build -o /dev/null .; Remove-Item -Force terminal.test; $env:GOOS=''
$env:GOOS='darwin'; go build -o /dev/null .; $env:GOOS=''
```
- **Resultado**: Compilação cruzada sem erros ou avisos para Linux e macOS.

### 3. Compilação do Executável Windows
```powershell
go build -ldflags="-s -w" -o bg3-guard.exe .
```
- **Resultado**: Binário `bg3-guard.exe` gerado com sucesso.
- Validação de comandos CLI: `.\bg3-guard.exe --version` e `.\bg3-guard.exe --help` funcionam perfeitamente sem auto-spawn quando chamados do terminal existente (pois a contagem de processos no console é $\ge 2$).
