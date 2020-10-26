# AWS Cost Comparison Tool

This is a simple CLI tool that wraps the AWS Cost Explorer APIs to be able to quickly identify cost anomalies between monthly bills.

## Requirements

* AWS Credentials Setup locally
* Access to Cost Explorer (`ce:*`)

## Installation

### Homebrew

```
brew tap rocketmiles/aws-cct https://github.com/rocketmiles/aws-cct
brew install aws-cct
```

### Through GitHub

Download from the [releases](https://github.com/rocketmiles/aws-cct/releases)

### Go Get

```
go get github.com/rocketmiles/aws-cct
```

## Usage

For full usage, see the help page with `aws-cct help`:

```
NAME:
   aws-cct - AWS Cost Comparison Tool

USAGE:
   aws-cct [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --start value        First month to compare (2020-01-01) (default: "2020-09-01")
   --end value          Second month to compare (2020-02-01) (default: "2020-10-01")
   --cost-metric value  Cost Metric to compare (NetAmortizedCost, UnblendedCost, etc.) (default: "NetAmortizedCost")
   --service value      Define a service to dig into
   --help, -h           show help (default: false)
```

*Simple usage*
```
aws-cct
```

*Check Unblended Costs*
```
aws-cct --cost-metric UnblendedCost
```

*Dig into EC2 costs*
You can get the string from the initial output. Simply copy the value in the "SERVICE" section and you can filter into that
```
aws-cct --service "Amazon Elastic Compute Cloud - Compute"
```

*Compare Older Months*
```
aws-cct --start 2020-08-01 --end 2020-09-01
```

## Local Development

Requires Go >= 1.15.3

Build with `go build`

You should see a local binary called `aws-cct` which you can use to interact with

## Credits

[AWS SDK for Go](https://docs.aws.amazon.com/sdk-for-go/api/service/costexplorer/)

[Urfav CLI Lib](https://github.com/urfave/cli/)

[go-pretty for table output](https://github.com/jedib0t/go-pretty)

## License

MIT
