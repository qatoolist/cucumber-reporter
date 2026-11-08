$n = Hash.new(0)
Given('ok') { }
Given('pend') { pending('todo') }
Given('flaky once') { $n[:f]+=1; raise 'flaky' if $n[:f]==1 }
Given('broken') { raise 'wip broken' }
