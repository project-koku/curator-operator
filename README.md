# curator-operator

## Generate reports as .CSV files for delivery to users

Start HTTP server

Connect to Postgres database

Write APIs for downloading CSV files. (Need to migrate existing python code to golang)  Implement API error handling 

User can download reports using below APIs

1. Time frame report

```
curl -XGET "http://localhost:5000/download?start=2021-08-22%2003:00:00.000000&end=2021-08-22%2004:00:00.000000"
```

2. Standard daily, weekly, monthly report

```
curl -XGET "http://localhost:5000/report?reportName=daily-report-sample&reportNamespace=report-system"
```
