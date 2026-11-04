-- Keep native liblipc ownership in this child; Go completes each lifecycle step.
local lipc = require('liblipclua')
local h = assert(lipc.init('org.kindlepowerful.library'))
local props = {}
for _, name in ipairs({'load','go','pause','unload'}) do
 local p = h:register_string_property(name, 'w')
 p.listener = function(key, value)
  io.stdout:write(key .. '\t' .. tostring(value) .. '\n'); io.stdout:flush()
 end
 props[#props+1] = p
end
h:subscribe('com.lab126.powerd', 'goingToScreenSaver', function()
 io.stdout:write('power\tsleep\n'); io.stdout:flush()
end)
h:subscribe('com.lab126.powerd', 'exitingScreenSaver', function()
 io.stdout:write('power\twake\n'); io.stdout:flush()
end)
io.stdout:write('ready\n'); io.stdout:flush()
while true do lipc.run_event_loop(math.huge) end
