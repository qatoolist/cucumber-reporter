$n = Hash.new(0)
Given('step ffp') { $n[:ffp]+=1; raise "ffp #{$n[:ffp]}" if $n[:ffp] < 3 }
Given('step fpend') { $n[:fp]+=1; raise 'fpend 1' if $n[:fp]==1; pending('now pending') }
Given('step fundef') { $n[:fu]+=1; raise 'fundef 1' if $n[:fu]==1; step 'this is not defined anywhere' }
Given('step ok') { }
Given('step always') { raise 'always fails' }
Given('step fff_p') { $n[:f3]+=1; raise "fff #{$n[:f3]}" if $n[:f3] < 4 }
