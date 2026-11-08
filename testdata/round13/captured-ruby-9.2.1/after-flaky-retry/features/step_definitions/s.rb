$n=Hash.new(0)
Given('ok') { }
After { |sc| $n[sc.name]+=1; raise "after #{sc.name}" if sc.name.include?('always') || $n[sc.name]==1 }
