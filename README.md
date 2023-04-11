# fileserver-challenge


**Background Context:**

In industry today, there’s no such thing as “just being a programmer”. Engineers are expected to own their application code from 
local development through production. This means developers are **on call** for their applications, and may be expected to 
live-debug and remediate bugs in production code. Engineers must understand the nitty-gritty details of how their code runs 
production, and will collect data on application performance, user experience (such as drop-off rates), and error rates, 
that they will use to make data-driven decisions on prioritization of technical debt, bug fixes, and new feature development.


In this challenge you’ll be presented with a backend “file server” that stores and returns files stored at provided locations. 
Your team does not maintain this application, so you do not have access to update its code. Instead, you must improve its 
performance by writing custom middleware, or by fine-tuning the surrounding system to maximize total service throughput. 
Like all production systems, this data-store has limited resources, so while you may be able to tweak how many resources are 
available to individual components, you may not exceed the limits set by the rules of this challenge. You may run more than one
file server if desired. 

**Here are the limits of your application:**

1. Each file server cannot maintain more than 10 total connections at any point in time.
2. All services, including the fileserver(s) have a hard aggregate limit of 2CPU cores
3. All services have a hard aggregate limit of 4GB Memory
4. You may run more than one file server, but you may not run more than 5. All file server share a single data volume.

> Beware, running more than 1 file server could result in data integrity challenges. Consider how you could prevent these challenges.


First, you must write a (at least one) middleware application that will serve file data to-and-from the backend file server(s). You may write your application middleware in **any language**, but **do not over index on language, as it is far less important to performance than you might expect. **

Your middleware MUST expose 3 REST API endpoints:

* PUT - /api/fileserver/fileName
    * Add a new file to the fileServer at location `fileName`
* GET - /api/fileserver/fileName
    * Retrieve the contents of the file at fileServer at location `fileName`
* DELETE - /api/fileserver/fileName
    * Delete the file stored at location `fileName`

Your application must bind itself to `0.0.0.0:8080` and this port must be exposed from its docker container so it 
is available to external connections. The challenge is to employ any-and-all known design patterns to maximize the 
performance of the file server. There must be a **single address exposed** that will receive 100% of traffic.


If desired, you may deploy open source third party web servers (such as Apache, or 
[Nginx](https://www.nginx.com/))  to run or support your application by providing load balancing.

**The Test:**

Your file server will be load tested, live, in front of the class, and aggregate performance and availability metrics 
will be displayed. The below metrics will aggregated:

* HTTP 2XX’s
* HTTP 5XX’s
* Average sustained throughput (requests / sec)
* Maximum achieved successful throughput
* Number of times invalid data was returned (i.e, I wrote X last, but was returned Y as a value)
* Edge case tests (consider what these edge cases could be)

**What you’ll be given:**

You will be provided with a simple python load-testing script that is designed to execute load tests against the provided 
API endpoints above. This script is **not the script that will be run in our live test**. It’s difficult to simulate real-world 
production traffic, so instead this script will serve as a base. You may tweak it however you like to collect performance data, 
or to modify the parameters by which it runs.


You will also be given a docker-compose.yml file that will start a single instance of the `file-server` application. You will need 
to update this file to add your middleware. You may modify this file however you like, but beware, you **must ensure your 
aggregate system resources adhere to the limits defined above.**
