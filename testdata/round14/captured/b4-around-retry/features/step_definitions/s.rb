# outcome "K": PLAN K:fail -> raise; K:failN -> fail first N calls in this run (counter); default pass
Given(/^outcome "([^"]*)"$/) do |k|
  v = PLAN[k] || 'pass'
  if v == 'fail'
    raise "planned failure #{k} (run #{ENV['RUNNO']})"
  elsif v =~ /^fail(\d+)$/
    n = bump(k)
    raise "planned flaky failure #{k} call #{n} (run #{ENV['RUNNO']})" if n <= $1.to_i
  elsif v == 'pending'
    pending "planned pending #{k}"
  end
end
Given(/^ok$/) { }
