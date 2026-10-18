# EzzyCraft

## Starting the server

Build the server with `build.bat`, then start it with an operator code:

```powershell
.\mcserver.exe -op-code "your-private-code"
```

Open the server address in a browser. In game, open **Options...**, enter the operator code, and choose **Unlock Server Settings** to import item/block definition JSON files. Without `-op-code`, server settings and definition imports remain disabled.

Shared definition JSON files are stored in `server_data\definitions` next to the server's working directory. The server creates this folder automatically and sends its definitions to connected players. Use HTTPS when exposing the server publicly so the operator code is protected in transit.

## Render

Do not put the operator code in the source code or in a public Render start command. In the Render dashboard, open your service's **Environment** settings and add a secret environment variable named `EZZYCRAFT_OP_CODE`. Set its value to a private code, save the change, and redeploy the service. The in-game **Options...** screen must be opened from that Render server's URL; enter the same value to unlock the import button.

The server also uses Render's `PORT` environment variable for its listen port. To keep imported definition files across redeploys, attach a persistent disk and set `-definitions-dir` to a folder on that disk using the service's start command.
