Given('a user with role {string}') { |r| }
Then('they have {int} items') { |n| raise "bad count #{n}" if n == 9 || n == 1 }
Given('a doc string:') { |s| }
Given('a table:') { |t| }
Given('rule setup') { }
