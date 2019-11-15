issues-to-go
---

A simple tool to download Github issues for offline reading. It uses the [GraphQL API v4](https://developer.github.com/v4/) and uses the package from [shurcooL/githubv4](https://github.com/shurcooL/githubv4) to do so.

Every occurrence of `#\d+` is replaced with a link to the referenced issue for easier navigation between issues.

Install
---

```shell script
go get -u github.com/S7evinK/issues-to-go
```

Usage
---

First you'll need a personal access token for Github, you can generate one [here](https://github.com/settings/tokens). The token needs access to `public_repo`, if you want to download issues from a private repository, you'll need to grant full access.

```shell script
issues-to-go downloads issues from Github for offline usage.
The default output format is Markdown. The issues are downloaded to a specified folder and to separate folders for open and closed issues.

After the first run a config file (.issues-to-go.yaml) will be created, subsequent runs from the same directory will use this file to determine the issues to download (if any).

Usage:
  issues-to-go [flags]

Examples:
You need to set an environment variable GITHUB_TOKEN with a personal access token in it. After the first run this token can also be put in the generated config file.

Download all issues associated with the repository "S7evinK/issues-to-go" to a folder "./issues":
        GITHUB_TOKEN=mysecrettoken issues-to-go -r S7evinK/issues-to-go

Download all issues to a specific folder "output":
        issues-to-go -r S7evinK/issues-to-go -o ./output

Flags:
      --all             Get open and closed issues. By default only open issues will be downloaded
      --config string   config file (default is .issues-to-go.yaml)
  -c, --count int       Sets the amount of issues/comments to fetch at once (default 100)
  -h, --help            help for issues-to-go
      --milestones      Create a separate folder with issues linked to milestones.
  -o, --output string   Output folder to download the issues to (default "./issues")
  -r, --repo string     Repository to download (eg: S7evinK/issues-to-go)
      --utc             Use UTC for dates. Defaults to false
```

Example output:
```shell script
$ cat issues/open/1.md
Test issue
---

Created by S7evinK on 2019-11-15 13:05:33 +0100 CET:

Hello World!

---

S7evinK commented on 2019-11-15 13:07:38 +0100 CET:

**This is a dummy comment.**

---

```
```shell script
$ tree issues
issues
├── closed
│   ├── 756.md
│   ├── 765.md
│   ├── 773.md
│   ├── 776.md
│   └── 780.md
└── open
    ├── 760.md
    ├── 761.md
    ├── 764.md
    ├── 767.md
    ├── 775.md
    ├── 806.md
    ├── 814.md
    ├── 815.md
    └── 820.md
```