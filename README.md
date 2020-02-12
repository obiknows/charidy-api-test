Charidy API Test
================

by Obinna (@obiknows)

1. Create 3 API dummy endpoints which are doing something that takes random amount of time (from 500ms to 1.5s):
1 GET 
2 POST:
- 1x POST is taking and parsing random JSON data, returns same data parsed with easy to read form (use indent).
- 1x POST is taking and parsing data in JSONAPI (https://jsonapi.org/) format (use google library for JSONAPI parsing), returns same as the first one.
Cover these 3 endpoints with simple rate limit - max 10 connections a second from one IP

Cover all 3 APIs with tests, what needs to be tested:
- that API works in general
- that valid JSON is being parsed
- invalid JSON returns error
- valid JSONAPI being parsed
- invalid JSONAPI returns error
- rate limit works:
-- less than 10 hits per second
-- exact 10 hits per second
-- more than 10 hits per second - should fail with "Too many requests" http code
