'''
This file checks for the DB utilization and updates the users via email if the DB has reached the limit
Author: Rajath (vanakudare.r@northeastern.edu)
'''

import time
import os
import json
import smtplib
import ssl
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart

def getEnvVars():
    print("Getting environment variables")
    email_user = os.getenv("EMAIL_USER")
    email_pass = os.getenv("EMAIL_PASS")
    email_recipients = parse_json(os.getenv("EMAIL_RECI"))
    limit = os.getenv("MEM_LIMIT")
    
    try:
        limit = int(limit)
    except:
        print("Unable to parse limit... Using default value of 75")
        limit = 75

    if email_user == "":
        print("Email user not found... Exiting...")
        exit(0)
    
    return email_user, email_pass, email_recipients, limit

def send_email(sender, password, receiver, usage):
    if not receiver:
        return

    ctx = ssl.create_default_context()

    message = MIMEMultipart("alternative")
    message["Subject"] = "DB Utilization exceeds limit"
    message["From"] = sender
    message["To"] = ", ".join(receiver)
    text = "Warning: PSQL DB for Curator Reports is {}% full, you may lose reports if the DB has no space left.".format(str(usage))

    mail_body = MIMEText(text, "plain")

    message.attach(mail_body)

    with smtplib.SMTP_SSL("smtp.gmail.com", port=465, context=ctx) as server:
        server.login(sender, password)
        server.sendmail(sender, receiver, message.as_string())

def check_usage():
    usage = os.popen("df /var/lib/postgresql/data | awk 'END {print $5}'").read()
    usage = usage.replace("%", "")
    return int(usage)

def parse_json(email_recipients):
    try:
        EMAIL_RECIPIENTS = json.loads(email_recipients)
    except Exception as err:
        print("Failed to parse COST_MGMT_RECIPIENTS: ", err)
        exit(-1)
    return EMAIL_RECIPIENTS

if __name__ == "__main__":
    email_user, email_pass, email_recipients, limit = getEnvVars()
    while True:
        usage = check_usage()
        print("Current DB usage: ", usage)
        if usage >= limit:
            print("Exceeding the limit")
            for curr_user_email, email_item in email_recipients.items():
                print(f"User info: {curr_user_email, email_item}.")
                email_addrs = [curr_user_email] + email_item.get("cc", [])
                send_email(email_user, email_pass, email_addrs, usage)
            time.sleep(86400)
        else:
            print("Not exceedig the limit, rechecking in 60 seconds")
            time.sleep(60) #change value here when building the container if required to check less or more often
        