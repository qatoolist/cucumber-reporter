Given('feature background') { }
Given('rule background ok') { }
Given('rule background fails') { raise 'rule bg boom' }
Then('pass') { }
Then('fail') { raise 'example boom' }
