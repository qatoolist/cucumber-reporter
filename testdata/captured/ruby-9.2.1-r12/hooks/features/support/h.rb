Before('@before_fails') { raise 'before hook boom' }
After('@after_fails') { raise 'after hook boom' }
AfterStep('@afterstep_fails') { raise 'afterstep boom' }
