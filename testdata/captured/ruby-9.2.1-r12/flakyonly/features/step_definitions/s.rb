$n = Hash.new(0)
Given('ok') { }
Given('attempt step c') { $n[:c]+=1; raise 'first c' if $n[:c]==1 }
