variable "DOMAIN_NAME" {
  description = "OpenStack domain name"
  type        = string
}

variable "TENANT_NAME" {
  description = "OpenStack project name"
  type        = string
}

variable "USER_NAME" {
  description = "OpenStack user name"
  type        = string
  default     = "" # not needed if application credentials are used
}

variable "PASSWORD" {
  description = "OpenStack password"
  type        = string
  default     = "" # not needed if application credentials are used
}

variable "APPLICATION_CREDENTIAL_ID" {
  description = "OpenStack application credential id"
  type        = string
  default     = "" # not needed if username/password are used
}

variable "APPLICATION_CREDENTIAL_SECRET" {
  description = "OpenStack application credential secret"
  type        = string
  default     = "" # not needed if username/password are used
}
