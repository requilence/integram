# go-bitbucket

It is Bitbucket-API library for golang.

Support Bitbucket-API Version 2.0. 

And the response type is json format defined Bitbucket-API.

- ref) <https://confluence.atlassian.com/display/BITBUCKET/Version+2>

## Install

```
go get github.com/ktrysmt/go-bitbucket
```

## How to use

```
import "github.com/ktrysmt/go-bitbucket"
```

## Godoc

- <http://godoc.org/github.com/ktrysmt/go-bitbucket>


## Example

```
package main

import (
        "github.com/ktrysmt/go-bitbucket" 
        "fmt"
)

func main() {

        c := bitbucket.NewBasicAuth("username", "password")

        opt := bitbucket.PullRequestsOptions{
                Owner:      "your-team",
                Repo_slug:  "awesome-project",
                Source_branch: "develop",
                Destination_branch: "master",
                Title: "fix bug. #9999",
                Close_source_branch: true
        }
        res := c.Repositories.PullRequests.Create(opt)

        fmt.Println(res) // receive the data as json format
}
```

## License

MIT

## Author

[ktrysmt](https://github.com/ktrysmt)
