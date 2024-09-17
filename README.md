# seleniumctl

## Motivation

I hate writing bespoke selenium code.
It's boring and the API is ugly and very hard to remember.

This tool solves this, it is a cli that consumes a json format which encodes selenium steps.
It comes with a browser extension that records user interactions and formats it, so it fits into the format mentioned.

Beware, the browser extension is not feature complete but it does a pretty good job already.
Also, there is only an extension written for firefox.

## Usage


Compile and install via:

```
go install github.com/4thel00z/seleniumctl/...@latest
```


```
echo '[
  {
    "action": "navigate",
    "url": "https://google.de",
    "timestamp": 1726532349420
  }
] | seleniumctl'
```

Optional: Install the browser extension by going to [the debugging panel](about:debugging#/runtime/this-firefox).


## License

This project is licensed under the GPL-3 license.
