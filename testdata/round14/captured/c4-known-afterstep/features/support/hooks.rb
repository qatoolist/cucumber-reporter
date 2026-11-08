AfterStep do |result, step|
  raise "afterstep failure" if ENV['AS'] == '1' && @afterstep_done.nil? && (@afterstep_done = true)
end
