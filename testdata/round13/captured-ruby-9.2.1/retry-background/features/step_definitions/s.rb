$n = Hash.new(0)
Before { |sc| $cur = sc.name }
Given('the env is flaky per scenario') { $n[$cur]+=1; raise "bg down #{$cur} #{$n[$cur]}" if $cur == 'Bg always fails' || $n[$cur]==1 }
Given(/^bg target (\w)$/) { |x| }
Then('it works') { }
