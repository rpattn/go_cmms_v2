# 1) Install tools (once)
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
.\scripts\install.ps1

# 2) Set your env (this session)
. .\scripts\env.ps1

# 3) Run migrations
.\scripts\migrate-up.ps1

# 4) Generate sqlc code
.\scripts\sqlc-generate.ps1

# 5) Run the server
.\scripts\run.ps1
