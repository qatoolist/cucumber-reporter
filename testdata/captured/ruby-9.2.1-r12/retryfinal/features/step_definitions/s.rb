$n = Hash.new(0)
Given('attempt step a') { $n[:a]+=1; raise 'first a' if $n[:a]==1; pending('now pending') }
Given('attempt step b') { $n[:b]+=1; raise 'first b' if $n[:b]==1; skip_this_scenario }
Given('attempt step c') { $n[:c]+=1; raise 'first c' if $n[:c]==1 }
