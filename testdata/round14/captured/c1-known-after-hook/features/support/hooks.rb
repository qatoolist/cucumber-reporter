After do |scenario|
  raise "after hook failure" if scenario.name == 'Known one'
end
