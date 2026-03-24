

The local repo structure
```
agentmarket/
├── backend/               # Go source code
│   └── router.go
├── ops/                   # Podman and Caddy configs
│   ├── webapp.container
│   └── Caddyfile
├── site/                  # Marketing Site Source (Hugo/HTML/Markdown)
│   ├── index.html         # example.com/
│   ├── pricing/index.html # example.com/pricing
│   └── static/            # example.com/static/
│       └── logo.png       
└── frontend/              # Svelte App Source
    ├── src/               
    └── static/            # Virtual root for the Svelte app
        ├── favicon.ico    # example.com/favicon.ico
        └── images/
            └── icon.svg   # example.com/images/icon.svg
```

On Linux server
```
/home/<service account name>/agentmarket/
│
├── site/                  <-- CADDY SERVES THIS DIRECTLY
│   ├── index.html         
│   ├── pricing/index.html 
│   └── static/            
│       └── logo.png       
│
└── app/                   <-- GO (via PODMAN) SERVES THIS
    ├── index.html         # The Svelte fallback SPA file
    ├── _app/              # Svelte's compiled JS/CSS
    ├── favicon.ico        
    └── images/            
        └── icon.svg
```

Caddy handles certain routes
*  `/site`   --> `./site`
*  `/static` --> `./site/static`
*  `/blog`   --> `./site/blog`

```
	handle_path /site/* {
		root * /home/webapp/agentmarket/site
		file_server
	}

	@site_files path /static/* /about /pricing /blog*
	handle @site_files {
		root * /home/webapp/agentmarket/site
		file_server
	}
```









