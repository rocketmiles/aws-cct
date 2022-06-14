# AWS Cost Comparison Tool

This is a simple CLI tool that wraps the AWS Cost Explorer APIs to be able to quickly identify cost anomalies between monthly bills.

## Use Cases

* Comparing costs between two different months
* Creating a CSV report to show changes in the bill
* Diagnosing which specific line item changed the most to identify anomalies

## Requirements

* AWS Credentials Setup locally
* Access to Cost Explorer APIs (`ce:*`)

## Installation

### Homebrew

```bash
brew tap rocketmiles/aws-cct https://github.com/rocketmiles/aws-cct
brew install aws-cct
```

Updating
```bash
brew upgrade aws-cct
```

### Through GitHub

Download from the [releases](https://github.com/rocketmiles/aws-cct/releases)

### Go Get

```bash
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
   --start value        First month to compare (2020-01-01) (default: "2021-09-01")
   --end value          Second month to compare (2020-02-01) (default: "2021-10-01")
   --cost-metric value  Cost Metric to compare (NetAmortizedCost, UnblendedCost, etc.) (default: "NetAmortizedCost")
   --service value      Define a service to dig into
   --tag value          Tag value to filter results (app=web, env=prod, etc.)
   --sort value         Column to sort results on (name, start, end, delta, deltapercent) (default: "name")
   --sort-order value   Order to sort in (asc or desc) (default: "asc")
   --output value       Output format (supported formats: table, csv) (default: "table")
   --help, -h           show help (default: false)
```

*Simple usage*
```bash
aws-cct
```

*Check Unblended Costs*
```bash
aws-cct --cost-metric UnblendedCost
```

*Dig into EC2 costs*

You can get the string from the initial output. Simply copy the value in the "SERVICE" section and you can filter into that
```bash
aws-cct --service "Amazon Elastic Compute Cloud - Compute"
```

*Filter by tags*

You can get filter costs by tag, to return costs for resources that match all specified tag values.
```bash
aws-cct --tag app=widgetizer --tag env=production
```

*Compare Older Months*
```bash
aws-cct --start 2020-08-01 --end 2020-09-01
```

*Sort on a column*

You can sort on any column, ascending or descending, for example to see the largest deltas first.
```bash
aws-cct --sort delta --sort-order desc
```

*Output in CSV format*

This will output in a CSV friendly format and you can utilize this to do analysis or for reporting.
```bash
aws-cct --output csv
```

*Diagnose where the largest cost increase is coming from*

Between March & April 2022:

First, figure out which service has the largest cost
```bash
aws-cct --start 2022-03-01 --end 2022-04-01 --sort delta --sort-order desc
```

Then, copy that full service name and dig into which specific line item within that service increased the most.

In this example, the EC2 costs are high and we want to see what specifically increased.
```bash
aws-cct --start 2022-03-01 --end 2022-04-01 --sort delta --sort-order desc --service "Amazon Elastic Compute Cloud - Compute"
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
