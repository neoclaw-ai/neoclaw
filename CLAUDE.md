# BetterClaw
A better version of OpenClaw. A lightweight personal AI assistant.
80% of the features of OpenClaw, none of the headaches.

## Philosophy
- Unix Philosophy. Small sharp tools. 
- Use idiomatic clean go whenever possible. Try to use established patterns rather than reinventing your own. 
- 80/20 rule applies here. We do not need all of the bells and whistles. We need something that is good enough. 
- This is a small weekend project MVP not a massive app. Be lean and efficient. Do more with less.  
- Do not reinvent the wheel if there is already a tested and popular library available that does what we need, use it.
- The code should be easy for a human to read and understand. 
- Make sure to write tests for important functionality. We do not need 100% test coverage. 

## Main Goals
1. Easy to install, no external dependencies, a single go binary. 
2. Keep configuration easy for the user. Prefer sensible defaults without requiring the user to explicitly configure when possible. 
3. Security first by default. File writes isolated to a single working directory. User approves all URLs that are accessing. 
4. Optimize for lower token costs and efficient token usage. Do not make LLM calls when local processing will suffice. 
