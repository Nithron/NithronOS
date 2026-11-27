import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card-enhanced'
import { BackIcon, ConnectionIcon, DiskIcon, HomeIcon, SearchIcon, SecurityIcon, ServerIcon, WarningIcon } from '@/components/icons'
import { motion } from 'framer-motion'

export function NotFound() {
  const navigate = useNavigate()

  const quickLinks = [
    { 
      icon: DiskIcon, 
      label: 'Storage', 
      path: '/storage',
      description: 'Manage your storage pools'
    },
    { 
      icon: ServerIcon, 
      label: 'Apps', 
      path: '/apps',
      description: 'Browse installed applications'
    },
    { 
      icon: ConnectionIcon, 
      label: 'Network', 
      path: '/settings/network',
      description: 'Configure network settings'
    },
    { 
      icon: SecurityIcon, 
      label: 'Settings', 
      path: '/settings',
      description: 'System configuration'
    },
  ]

  return (
    <div className="min-h-screen w-full flex items-center justify-center p-4">
      <div className="max-w-2xl w-full space-y-8">
        {/* Main 404 Message */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          className="text-center space-y-4"
        >
          {/* Large 404 with gradient */}
          <div className="relative">
            <div className="text-[120px] md:text-[180px] font-bold leading-none">
              <span className="bg-gradient-to-r from-primary via-primary/80 to-primary/60 bg-clip-text text-transparent">
                404
              </span>
            </div>
            
            {/* Decorative elements */}
            <div className="absolute inset-0 flex items-center justify-center">
              <motion.div
                animate={{ 
                  rotate: 360,
                  scale: [1, 1.1, 1]
                }}
                transition={{ 
                  rotate: { duration: 20, repeat: Infinity, ease: "linear" },
                  scale: { duration: 3, repeat: Infinity, ease: "easeInOut" }
                }}
                className="absolute"
              >
                <div className="w-64 h-64 md:w-96 md:h-96 rounded-full border border-primary/10" />
              </motion.div>
              <motion.div
                animate={{ 
                  rotate: -360,
                  scale: [1, 0.9, 1]
                }}
                transition={{ 
                  rotate: { duration: 25, repeat: Infinity, ease: "linear" },
                  scale: { duration: 4, repeat: Infinity, ease: "easeInOut" }
                }}
                className="absolute"
              >
                <div className="w-48 h-48 md:w-72 md:h-72 rounded-full border border-primary/5" />
              </motion.div>
            </div>
          </div>

          {/* Error message */}
          <div className="space-y-2 relative z-10">
            <h1 className="text-2xl md:text-3xl font-semibold">
              Page Not Found
            </h1>
            <p className="text-muted-foreground max-w-md mx-auto">
              The page you're looking for doesn't exist or has been moved. 
              Let's get you back on track.
            </p>
          </div>

          {/* Action buttons */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.3 }}
            className="flex flex-col sm:flex-row gap-3 justify-center pt-4"
          >
            <Button
              size="lg"
              onClick={() => navigate(-1)}
              variant="outline"
              className="min-w-[140px]"
            >
              <BackIcon className="h-4 w-4 mr-2" />
              Go Back
            </Button>
            <Button
              size="lg"
              onClick={() => navigate('/')}
              className="min-w-[140px]"
            >
              <HomeIcon className="h-4 w-4 mr-2" />
              Dashboard
            </Button>
          </motion.div>
        </motion.div>

        {/* Quick Links */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
        >
          <Card>
            <div className="p-6">
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
                <SearchIcon className="h-5 w-5" />
                Quick Links
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {quickLinks.map((link, index) => (
                  <motion.button
                    key={link.path}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: 0.5 + index * 0.1 }}
                    onClick={() => navigate(link.path)}
                    className="flex items-start gap-3 p-3 rounded-lg hover:bg-muted/50 transition-colors text-left group"
                  >
                    <div className="mt-1">
                      <link.icon className="h-5 w-5 text-primary group-hover:scale-110 transition-transform" />
                    </div>
                    <div>
                      <div className="font-medium">{link.label}</div>
                      <div className="text-sm text-muted-foreground">
                        {link.description}
                      </div>
                    </div>
                  </motion.button>
                ))}
              </div>
            </div>
          </Card>
        </motion.div>

        {/* Technical Details (Optional) */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.7 }}
          className="text-center"
        >
          <div className="inline-flex items-center gap-2 px-3 py-2 rounded-lg bg-muted/30 text-sm text-muted-foreground">
            <WarningIcon className="h-4 w-4" />
            <span>Error Code: 404 | Resource Not Found</span>
          </div>
        </motion.div>
      </div>

      {/* Background decoration */}
      <div className="fixed inset-0 -z-10 overflow-hidden">
        <motion.div
          animate={{ 
            x: [0, 100, 0],
            y: [0, -100, 0]
          }}
          transition={{ 
            duration: 20,
            repeat: Infinity,
            ease: "easeInOut"
          }}
          className="absolute -top-1/2 -right-1/2 w-[800px] h-[800px] rounded-full bg-gradient-to-br from-primary/5 to-transparent blur-3xl"
        />
        <motion.div
          animate={{ 
            x: [0, -100, 0],
            y: [0, 100, 0]
          }}
          transition={{ 
            duration: 25,
            repeat: Infinity,
            ease: "easeInOut"
          }}
          className="absolute -bottom-1/2 -left-1/2 w-[800px] h-[800px] rounded-full bg-gradient-to-tr from-primary/5 to-transparent blur-3xl"
        />
      </div>
    </div>
  )
}
