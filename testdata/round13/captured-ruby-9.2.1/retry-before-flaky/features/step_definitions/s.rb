$n=Hash.new(0)
Given('ok') { }
Before { |sc| $n[sc.name]+=1; raise "before #{sc.name} #{$n[sc.name]}" if (sc.name.include?('always')) || (sc.name.include?('once') && $n[sc.name]==1) }
