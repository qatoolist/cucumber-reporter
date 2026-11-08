Around('@ar') do |scenario, block|
  block.call
  raise "around after failure for #{scenario.name}" if scenario.name == 'Wrapped two'
end
